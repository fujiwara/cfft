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
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfrontkeyvaluestore"
)

var Stage = types.FunctionStageDevelopment

var Version = "dev"

type CFFT struct {
	config     *Config
	cloudfront *cloudfront.Client
	cfkvs      *cloudfrontkeyvaluestore.Client
	cfkvsArn   string
	envs       map[string]string
}

func New(ctx context.Context, config *Config) (*CFFT, error) {
	app := &CFFT{
		config: config,
		envs:   map[string]string{},
	}
	awscfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config, %w", err)
	}
	app.cloudfront = cloudfront.NewFromConfig(awscfg)
	app.cfkvs = cloudfrontkeyvaluestore.NewFromConfig(awscfg)

	if config.KVS != nil {
		res, err := app.cloudfront.DescribeKeyValueStore(ctx, &cloudfront.DescribeKeyValueStoreInput{
			Name: aws.String(config.KVS.Name),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe kvs %s, %w", config.KVS.Name, err)
		}
		app.envs["KVS_ID"] = aws.ToString(res.KeyValueStore.Id)
		app.envs["KVS_NAME"] = aws.ToString(res.KeyValueStore.Name)
		app.cfkvsArn = aws.ToString(res.KeyValueStore.ARN)
	}

	return app, nil
}

func (app *CFFT) TestFunction(ctx context.Context, opt TestCmd) error {
	code, err := app.config.FunctionCode()
	if err != nil {
		return fmt.Errorf("failed to load function code, %w", err)
	}
	etag, err := app.prepareFunction(ctx, app.config.Name, code, opt.CreateIfMissing)
	if err != nil {
		return fmt.Errorf("failed to prepare function, %w", err)
	}

	for _, testCase := range app.config.TestCases {
		if err := app.runTestCase(ctx, app.config.Name, etag, testCase); err != nil {
			return fmt.Errorf("failed to run test case %s, %w", testCase.Identifier(), err)
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

func (app *CFFT) prepareFunction(ctx context.Context, name string, code []byte, createIfMissing bool) (string, error) {
	var functionConfig *types.FunctionConfig
	var etag string

	res, err := app.cloudfront.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
		Name:  aws.String(name),
		Stage: Stage,
	})
	if err != nil {
		var notFound *types.NoSuchFunctionExists
		if !errors.As(err, &notFound) {
			return "", fmt.Errorf("failed to describe function, %w", err)
		}
		if createIfMissing {
			log.Printf("[info] function %s not found", name)
			etag, err = app.createFunction(ctx, name, code)
			if err != nil {
				return "", fmt.Errorf("failed to create function, %w", err)
			}
		} else {
			return "", fmt.Errorf("function %s not found. To create a new function, add --create-if-missing flag", name)
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
			if bytes.Equal(res.FunctionCode, code) {
				log.Println("[info] function code is not changed")
			} else {
				log.Println("[info] function code is changed. updating function...")
				res, err := app.cloudfront.UpdateFunction(ctx, &cloudfront.UpdateFunctionInput{
					Name:           aws.String(name),
					IfMatch:        aws.String(etag),
					FunctionCode:   code,
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
	log.Printf("[info] testing function %s with case %s...", name, c.Identifier())
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
	var out any
	if err := json.Unmarshal([]byte(*res.TestResult.FunctionOutput), &out); err != nil {
		return fmt.Errorf("failed to parse function output, %w", err)
	}
	prettyOutput, _ := json.MarshalIndent(out, "", "  ")
	prettyOutput = append(prettyOutput, '\n')
	os.Stdout.Write(prettyOutput)
	if failed {
		return errors.New("test failed")
	}
	if c.expect == nil {
		return nil
	}

	var rhs any
	if err := json.Unmarshal([]byte(*res.TestResult.FunctionOutput), &rhs); err != nil {
		return fmt.Errorf("failed to parse function output, %w", err)
	}
	var options []jsondiff.Option
	if c.ignore != nil {
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
		fmt.Print(coloredDiff(diff))
		return fmt.Errorf("expect and actual are not equal")
	} else {
		log.Println("[info] expect and actual are equal")
	}
	return nil
}
