package cfft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

type CFFT struct {
	cloudfront   *cloudfront.Client
	functionCode []byte
	testCases    []TestCase
}

type TestCase struct {
	Event  []byte
	Expect any
	Ignore string
}

type CLI struct {
	Name     string `arg:"" help:"function name" required:"true"`
	Function string `arg:"" help:"function code file" required:"true"`
	Event    string `arg:"" help:"event object file" required:"true"`
	Expect   string `arg:"" help:"expect object file" optional:"true"`
	Ignore   string `short:"i" long:"ignore" help:"ignore fields in the expect object by jq syntax"`
}

func Run(ctx context.Context) error {
	var cli CLI
	kong.Parse(&cli)

	awscfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config, %w", err)
	}
	app := &CFFT{
		cloudfront: cloudfront.NewFromConfig(awscfg),
	}
	app.functionCode, err = os.ReadFile(cli.Function)
	if err != nil {
		return fmt.Errorf("failed to read function code, %w", err)
	}
	testCase := TestCase{}

	testCase.Event, err = os.ReadFile(cli.Event)
	if err != nil {
		return fmt.Errorf("failed to read event object, %w", err)
	}
	var eventObject any
	if err := json.Unmarshal(testCase.Event, &eventObject); err != nil {
		return fmt.Errorf("failed to parse event object, %w", err)
	}
	if cli.Expect != "" {
		expectBytes, err := os.ReadFile(cli.Expect)
		if err != nil {
			return fmt.Errorf("failed to read expect object, %w", err)
		}
		if err := json.Unmarshal(expectBytes, &testCase.Expect); err != nil {
			return fmt.Errorf("failed to parse expect object, %w", err)
		}
	}
	testCase.Ignore = cli.Ignore
	app.testCases = append(app.testCases, testCase)

	return app.TestFunction(ctx, cli.Name)
}

func (app *CFFT) TestFunction(ctx context.Context, name string) error {
	var created bool
	var etag string
	var functionConfig *types.FunctionConfig
	res, err := app.cloudfront.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
		Name: aws.String(name),
	})
	if err != nil {
		var notFound types.EntityNotFound
		if errors.Is(err, &notFound) {
			return fmt.Errorf("failed to describe function, %w", err)
		}
		log.Printf("[info] function %s not found. create function", name)
		res, err := app.cloudfront.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
			Name:         aws.String(name),
			FunctionCode: app.functionCode,
			FunctionConfig: &types.FunctionConfig{
				Comment: aws.String("created by cfft"),
				Runtime: types.FunctionRuntimeCloudfrontJs20,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create function, %w", err)
		}
		log.Printf("[info] function %s created", name)
		etag = aws.ToString(res.ETag)
		created = true
	} else {
		log.Printf("[info] function %s found", name)
		etag = aws.ToString(res.ETag)
		functionConfig = res.FunctionSummary.FunctionConfig
	}

	if !created {
		if res, err := app.cloudfront.GetFunction(ctx, &cloudfront.GetFunctionInput{
			Name: aws.String(name),
		}); err != nil {
			return fmt.Errorf("failed to describe function, %w", err)
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
					return fmt.Errorf("failed to update function, %w", err)
				}
				etag = aws.ToString(res.ETag)
			}
		}
	}

	for _, testCase := range app.testCases {
		var failed bool
		res, err := app.cloudfront.TestFunction(ctx, &cloudfront.TestFunctionInput{
			Name:        aws.String(name),
			IfMatch:     aws.String(etag),
			Stage:       types.FunctionStageDevelopment,
			EventObject: testCase.Event,
		})
		if err != nil {
			return fmt.Errorf("failed to test function, %w", err)
		}
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
	}
	return nil
}
