package cfft

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/goccy/go-yaml"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

type DiffCmd struct {
	Live bool `cmd:"" help:"diff with LIVE stage"`
}

func (app *CFFT) DiffFunction(ctx context.Context, opt *DiffCmd) error {
	if err := app.diffFunctionConfig(ctx); err != nil {
		return err
	}
	stage := types.FunctionStageDevelopment
	if opt.Live {
		stage = types.FunctionStageLive
	}
	if err := app.diffFunctionCode(ctx, stage); err != nil {
		return err
	}
	return nil
}

func (app *CFFT) diffFunctionConfig(ctx context.Context) error {
	name := app.config.Name
	var remoteConfig *types.FunctionConfig
	var remote string
	res, err := app.cloudfront.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
		Name:  aws.String(name),
		Stage: Stage,
	})
	if err != nil {
		var notFound *types.NoSuchFunctionExists
		if errors.As(err, &notFound) {
			slog.Info(f("function %s not found", name))
		} else {
			return fmt.Errorf("failed to describe function, %w", err)
		}
	} else {
		slog.Debug(f("function %s found", name))
		remoteConfig = res.FunctionSummary.FunctionConfig
		remoteConfig.KeyValueStoreAssociations = nil // ignore kvs association
		remote = aws.ToString(res.ETag)
	}

	localConfig := &types.FunctionConfig{
		Comment: aws.String(app.config.Comment),
		Runtime: app.config.Runtime,
	}
	local := app.config.path

	remoteCode, _ := yaml.Marshal(remoteConfig)
	localCode, _ := yaml.Marshal(localConfig)

	if isSameCode(remoteCode, localCode) {
		slog.Info("function config is up-to-date")
		return nil
	}

	edits := myers.ComputeEdits(span.URIFromPath(remote), string(remoteCode), string(localCode))
	out := fmt.Sprint(gotextdiff.ToUnified(remote, local, string(remoteCode), edits))
	fmt.Print(coloredDiff(out))
	return nil
}

func (app *CFFT) diffFunctionCode(ctx context.Context, stage types.FunctionStage) error {
	name := app.config.Name
	var remoteCode []byte
	res, err := app.cloudfront.GetFunction(ctx, &cloudfront.GetFunctionInput{
		Name:  aws.String(name),
		Stage: stage,
	})
	if err != nil {
		var notFound *types.NoSuchFunctionExists
		if errors.As(err, &notFound) {
			slog.Info(f("function %s not found in %s stage", name, stage))
		} else {
			return fmt.Errorf("failed to describe function, %w", err)
		}
	} else {
		slog.Debug(f("function %s found", name))
		remoteCode = res.FunctionCode
	}

	var remote string
	if res != nil {
		remote = aws.ToString(res.ETag)
	}
	localCode, err := app.config.FunctionCode(ctx)
	if err != nil {
		return fmt.Errorf("failed to read function code, %w", err)
	}

	if isSameCode(localCode, remoteCode) {
		slog.Info("function code is up-to-date")
		return nil
	}

	edits := myers.ComputeEdits(span.URIFromPath(remote), string(remoteCode), string(localCode))
	out := fmt.Sprint(gotextdiff.ToUnified(remote, "local", string(remoteCode), edits))
	fmt.Print(coloredDiff(out))
	return nil
}
