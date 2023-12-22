package cfft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aereal/jsondiff"
	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/itchyny/gojq"
)

var Stage = types.FunctionStageDevelopment

type CFFT struct {
	cloudfront   *cloudfront.Client
	functionCode []byte
	testCases    []*TestCase
}

type TestCase struct {
	EventFile  string
	ExpectFile string
	IgnoreStr  string

	event  []byte
	expect any
	ignore *gojq.Query
}

func Run(ctx context.Context) error {
	cli := &CLI{}
	kong.Parse(cli)

	log.Printf("[info] loading function %s from %s", cli.Name, cli.Function)
	functionCode, err := os.ReadFile(cli.Function)
	if err != nil {
		return fmt.Errorf("failed to read function code, %w", err)
	}
	testCase, err := cli.NewTestCase(ctx)
	if err != nil {
		return fmt.Errorf("failed to create test case, %w", err)
	}

	app, err := NewApp(ctx, functionCode, testCase)
	if err != nil {
		return fmt.Errorf("failed to create app, %w", err)
	}
	return app.TestFunction(ctx, cli.Name)
}

func NewApp(ctx context.Context, functionCode []byte, testCases ...*TestCase) (*CFFT, error) {
	app := &CFFT{
		functionCode: functionCode,
		testCases:    testCases,
	}
	awscfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config, %w", err)
	}
	app.cloudfront = cloudfront.NewFromConfig(awscfg)
	return app, nil
}

func (app *CFFT) TestFunction(ctx context.Context, name string) error {
	etag, err := app.prepareFunction(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to prepare function, %w", err)
	}

	for _, testCase := range app.testCases {
		if err := app.runTestCase(ctx, name, etag, testCase); err != nil {
			return fmt.Errorf("failed to run test case, %w", err)
		}
	}
	return nil
}

func (app *CFFT) createFunction(ctx context.Context, name string, code []byte) (string, error) {
	log.Printf("[info] creating function %s...", name)
	res, err := app.cloudfront.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:         aws.String(name),
		FunctionCode: code,
		FunctionConfig: &types.FunctionConfig{
			Comment: aws.String("created by cfft"),
			Runtime: types.FunctionRuntimeCloudfrontJs20,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create function, %w", err)
	}
	log.Printf("[info] function %s created", name)
	return aws.ToString(res.ETag), nil
}

func (app *CFFT) prepareFunction(ctx context.Context, name string) (string, error) {
	var functionConfig *types.FunctionConfig
	var etag string

	res, err := app.cloudfront.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
		Name:  aws.String(name),
		Stage: Stage,
	})
	if err != nil {
		var notFound types.EntityNotFound
		if errors.Is(err, &notFound) {
			return "", fmt.Errorf("failed to describe function, %w", err)
		}
		log.Printf("[info] function %s not found", name)
		etag, err = app.createFunction(ctx, name, app.functionCode)
		if err != nil {
			return "", fmt.Errorf("failed to create function, %w", err)
		}
	} else {
		log.Printf("[info] function %s found", name)
		functionConfig = res.FunctionSummary.FunctionConfig
		if res, err := app.cloudfront.GetFunction(ctx, &cloudfront.GetFunctionInput{
			Name:  aws.String(name),
			Stage: Stage,
		}); err != nil {
			return "", fmt.Errorf("failed to describe function, %w", err)
		} else {
			etag = aws.ToString(res.ETag)
			if bytes.Equal(res.FunctionCode, app.functionCode) {
				log.Println("[info] function code is not changed")
			} else {
				fmt.Println("[info] function code is changed. updating function...")
				res, err := app.cloudfront.UpdateFunction(ctx, &cloudfront.UpdateFunctionInput{
					Name:           aws.String(name),
					IfMatch:        aws.String(etag),
					FunctionCode:   app.functionCode,
					FunctionConfig: functionConfig,
				})
				if err != nil {
					return "", fmt.Errorf("failed to update function, %w", err)
				}
				etag = aws.ToString(res.ETag)
			}
		}
	}
	return etag, nil
}

func (app *CFFT) runTestCase(ctx context.Context, name, etag string, c *TestCase) error {
	log.Printf("[info] testing function %s with event:%s expect:%s ignore:%s", name, c.EventFile, c.ExpectFile, c.IgnoreStr)
	res, err := app.cloudfront.TestFunction(ctx, &cloudfront.TestFunctionInput{
		Name:        aws.String(name),
		IfMatch:     aws.String(etag),
		Stage:       Stage,
		EventObject: c.event,
	})
	if err != nil {
		return fmt.Errorf("failed to test function, %w", err)
	}
	var failed bool
	if errMsg := aws.ToString(res.TestResult.FunctionErrorMessage); errMsg != "" {
		log.Printf("[error] %s", errMsg)
		failed = true
	}
	log.Printf("[info] ComputeUtilization:%s", aws.ToString(res.TestResult.ComputeUtilization))
	for _, l := range res.TestResult.FunctionExecutionLogs {
		log.Println(l)
	}
	fmt.Println(aws.ToString(res.TestResult.FunctionOutput))
	if failed {
		return errors.New("test failed")
	}
	if c.ExpectFile == "" {
		return nil
	}

	var rhs any
	if err := json.Unmarshal([]byte(*res.TestResult.FunctionOutput), &rhs); err != nil {
		return fmt.Errorf("failed to parse function output, %w", err)
	}
	var options []jsondiff.Option
	if c.IgnoreStr != "" {
		options = append(options, jsondiff.Ignore(c.ignore))
	}
	diff, err := jsondiff.Diff(
		&jsondiff.Input{Name: "expect", X: c.expect},
		&jsondiff.Input{Name: "actual", X: rhs},
		options...,
	)
	if err != nil {
		return fmt.Errorf("failed to diff, %w", err)
	}
	if diff != "" {
		return fmt.Errorf("expect and actual are not equal:\n%s", diff)
	} else {
		log.Println("[info] expect and actual are equal")
	}
	return nil
}
