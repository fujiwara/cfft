package cfft

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

type PublishCmd struct {
}

func (app *CFFT) PublishFunction(ctx context.Context, opt PublishCmd) error {
	name := app.config.Name
	var etag string
	var functionCode []byte
	if res, err := app.cloudfront.GetFunction(ctx, &cloudfront.GetFunctionInput{
		Name:  aws.String(name),
		Stage: Stage,
	}); err != nil {
		var notFound *types.NoSuchFunctionExists
		if errors.As(err, &notFound) {
			return fmt.Errorf("function %s not found. please run `cfft test --create-if-missing` before publish", name)
		}
		return fmt.Errorf("failed to describe function, %w", err)
	} else {
		etag = aws.ToString(res.ETag)
		functionCode = res.FunctionCode
	}
	log.Printf("[info] function %s found", name)
	if !bytes.Equal(app.config.functionCode, functionCode) {
		return fmt.Errorf("function code is not up-to-date. please run `cfft diff` and `cfft test` before publish")
	}

	log.Printf("[info] publishing function %s...", name)
	if _, err := app.cloudfront.PublishFunction(ctx, &cloudfront.PublishFunctionInput{
		Name:    aws.String(name),
		IfMatch: aws.String(etag),
	}); err != nil {
		return fmt.Errorf("failed to publish function, %w", err)
	}
	log.Printf("[info] function %s published successfully", name)
	return nil
}
