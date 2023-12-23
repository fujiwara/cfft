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
	"github.com/itchyny/gojq"
)

var Stage = types.FunctionStageDevelopment

var Version = "dev"

type CFFT struct {
	config     *Config
	cloudfront *cloudfront.Client
}

type TestCase struct {
	Name   string `json:"name" yaml:"name"`
	Event  string `json:"event" yaml:"event"`
	Expect string `json:"expect" yaml:"expect"`
	Ignore string `json:"ignore" yaml:"ignore"`

	id     int
	event  []byte
	expect any
	ignore *gojq.Query
}

func (c *TestCase) Identifier() string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("[%d]", c.id)
}

func (c *TestCase) Setup(ctx context.Context, readFile func(string) ([]byte, error)) error {
	var event any
	if err := json.Unmarshal([]byte(c.Event), &event); err != nil {
		// event is not JSON string
		eventBytes, err := readFile(c.Event)
		if err != nil {
			return fmt.Errorf("failed to read event object, %w", err)
		}
		c.event = eventBytes
	} else {
		c.event = []byte(c.Event)
	}
	if len(c.event) == 0 {
		return errors.New("event is empty")
	}

	if len(c.Expect) > 0 {
		// expect is optional
		var expect any
		if err := json.Unmarshal([]byte(c.Expect), &expect); err != nil {
			// expect is not JSON string
			expectBytes, err := readFile(c.Expect)
			if err != nil {
				return fmt.Errorf("failed to read expect object, %w", err)
			}
			if err := json.Unmarshal(expectBytes, &c.expect); err != nil {
				return fmt.Errorf("failed to parse expect object, %w", err)
			}
		} else {
			c.expect = []byte(c.Expect)
		}
	}

	if len(c.Ignore) > 0 {
		// ignore is optional
		q, err := gojq.Parse(c.Ignore)
		if err != nil {
			return fmt.Errorf("failed to parse ignore query, %w", err)
		}
		c.ignore = q
	}
	return nil
}

func New(ctx context.Context, config *Config) (*CFFT, error) {
	app := &CFFT{
		config: config,
	}
	awscfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config, %w", err)
	}
	app.cloudfront = cloudfront.NewFromConfig(awscfg)
	return app, nil
}

func (app *CFFT) TestFunction(ctx context.Context, creaetIfMissing bool) error {
	etag, err := app.prepareFunction(ctx, app.config.Name, app.config.functionCode, creaetIfMissing)
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
		var notFound types.EntityNotFound
		if errors.Is(err, &notFound) {
			return "", fmt.Errorf("failed to describe function, %w", err)
		}
		if !createIfMissing {
			log.Printf("[info] function %s not found", name)
			etag, err = app.createFunction(ctx, name, code)
			if err != nil {
				return "", fmt.Errorf("failed to create function, %w", err)
			}
		} else {
			return "", fmt.Errorf("function %s not found", name)
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
		return fmt.Errorf("expect and actual are not equal:\n%s", diff)
	} else {
		log.Println("[info] expect and actual are equal")
	}
	return nil
}
