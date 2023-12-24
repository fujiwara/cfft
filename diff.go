package cfft

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

type DiffCmd struct{}

func (app *CFFT) DiffFunction(ctx context.Context, opt DiffCmd) error {
	name := app.config.Name
	var remoteCode []byte
	res, err := app.cloudfront.GetFunction(ctx, &cloudfront.GetFunctionInput{
		Name:  aws.String(name),
		Stage: Stage,
	})
	if err != nil {
		var notFound *types.NoSuchFunctionExists
		if errors.As(err, &notFound) {
			log.Printf("[info] function %s not found", name)
		} else {
			return fmt.Errorf("failed to describe function, %w", err)
		}
	} else {
		log.Printf("[info] function %s found", name)
		remoteCode = res.FunctionCode
	}

	var remote string
	if res != nil {
		remote = aws.ToString(res.ETag)
	}
	local := app.config.Function
	localCode := app.config.functionCode

	edits := myers.ComputeEdits(span.URIFromPath(remote), string(remoteCode), string(localCode))
	out := fmt.Sprint(gotextdiff.ToUnified(remote, local, string(remoteCode), edits))
	fmt.Print(coloredDiff(out))

	return nil
}
