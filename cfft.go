package cfft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/aereal/jsondiff"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfrontkeyvaluestore"
	"github.com/shogo82148/go-retry"
)

var Stage = types.FunctionStageDevelopment

var Version = "dev"

var logLevel = new(slog.LevelVar)

const DefaultRuntime = types.FunctionRuntimeCloudfrontJs20

type CFFT struct {
	config     *Config
	cloudfront *cloudfront.Client
	cfkvs      *cloudfrontkeyvaluestore.Client
	cfkvsArn   string
	envs       map[string]string
	stdout     io.Writer
}

func (app *CFFT) SetStdout(w io.Writer) {
	app.stdout = w
}

func New(ctx context.Context, config *Config) (*CFFT, error) {
	app := &CFFT{
		config: config,
		envs:   map[string]string{},
		stdout: os.Stdout,
	}

	// CloudFront region is fixed to us-east-1
	awscfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config, %w", err)
	}
	app.cloudfront = cloudfront.NewFromConfig(awscfg)
	app.cfkvs = cloudfrontkeyvaluestore.NewFromConfig(awscfg)

	return app, nil
}

func (app *CFFT) prepareKVS(ctx context.Context, create bool) error {
	if app.config == nil || app.config.KVS == nil {
		return nil
	}
	name := app.config.KVS.Name
	res, err := app.cloudfront.DescribeKeyValueStore(ctx, &cloudfront.DescribeKeyValueStoreInput{
		Name: aws.String(name),
	})
	if err == nil { // found
		slog.Debug(f("kvs %s found", app.config.KVS.Name))
		app.envs["KVS_ID"] = aws.ToString(res.KeyValueStore.Id)
		app.envs["KVS_NAME"] = aws.ToString(res.KeyValueStore.Name)
		app.cfkvsArn = aws.ToString(res.KeyValueStore.ARN)
		return nil
	}
	// not found
	if !create {
		slog.Warn(f("failed to describe kvs %s, %s", name, err))
		return fmt.Errorf("kvs %s not found. To create a new kvs, add --create-if-missing flag", name)
	}

	// create
	slog.Info(f("kvs %s not found, creating...", name))
	if res, err := app.cloudfront.CreateKeyValueStore(ctx, &cloudfront.CreateKeyValueStoreInput{
		Name:    aws.String(name),
		Comment: aws.String(app.config.Comment),
	}); err != nil {
		return fmt.Errorf("failed to create kvs %s, %w", name, err)
	} else {
		app.cfkvsArn = aws.ToString(res.KeyValueStore.ARN)
		app.envs["KVS_ID"] = aws.ToString(res.KeyValueStore.Id)
		app.envs["KVS_NAME"] = aws.ToString(res.KeyValueStore.Name)
	}

	// kvs is not ready immediately after creation. wait for a while
	var policy = retry.Policy{
		MinDelay: 1 * time.Second,
		MaxDelay: 10 * time.Second,
		MaxCount: 30,
	}
	retrier := policy.Start(ctx)
	for retrier.Continue() {
		if err := app.waitForKVSReady(ctx, name); err == nil {
			slog.Info(f("kvs %s created", name))
			return nil
		}
	}
	return fmt.Errorf("failed to create kvs %s, timed out", name)
}

func (app *CFFT) waitForKVSReady(ctx context.Context, name string) error {
	res, err := app.cloudfront.DescribeKeyValueStore(ctx, &cloudfront.DescribeKeyValueStoreInput{
		Name: aws.String(name),
	})
	if err != nil {
		return retry.MarkPermanent(fmt.Errorf("failed to describe kvs %s, %w", name, err))
	}
	switch s := aws.ToString(res.KeyValueStore.Status); s {
	case "READY":
		return nil
	default:
		err := fmt.Errorf("kvs %s is not ready yet. status: %s", name, s)
		slog.Info(err.Error())
		return err
	}
}

func (app *CFFT) TestFunction(ctx context.Context, opt *TestCmd) error {
	if err := opt.Setup(); err != nil {
		return err
	}
	code, err := app.config.FunctionCode(ctx)
	if err != nil {
		return fmt.Errorf("failed to load function code, %w", err)
	}
	etag, err := app.prepareFunction(ctx, app.config.Name, code, opt.CreateIfMissing)
	if err != nil {
		return fmt.Errorf("failed to prepare function, %w", err)
	}

	var pass, fail int
	var errs []error
	for _, testCase := range app.config.TestCases {
		if !opt.ShouldRun(testCase.Identifier()) {
			slog.Debug(f("skipping test case %s", testCase.Identifier()))
			continue
		}
		if err := app.runTestCase(ctx, app.config.Name, etag, testCase); err != nil {
			fail++
			e := fmt.Errorf("failed to run test case %s, %w", testCase.Identifier(), err)
			slog.Error(e.Error())
			errs = append(errs, e)
		} else {
			pass++
		}
	}
	if fail > 0 {
		slog.Info(f("%d testcases passed, %d testcases failed", pass, fail))
	} else {
		slog.Info(f("%d testcases passed", pass))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (app *CFFT) createFunction(ctx context.Context, name string, code []byte) (string, error) {
	slog.Info(f("creating function %s...", name))
	var kvsassociation *types.KeyValueStoreAssociations
	if app.cfkvsArn != "" {
		kvsassociation = &types.KeyValueStoreAssociations{
			Quantity: aws.Int32(1),
			Items: []types.KeyValueStoreAssociation{
				{KeyValueStoreARN: aws.String(app.cfkvsArn)},
			},
		}
	}
	res, err := app.cloudfront.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:         aws.String(name),
		FunctionCode: code,
		FunctionConfig: &types.FunctionConfig{
			Comment:                   aws.String(app.config.Comment),
			Runtime:                   app.config.Runtime,
			KeyValueStoreAssociations: kvsassociation,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create function, %w", err)
	}
	slog.Info(f("function %s created", name))
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
			slog.Info(f("function %s not found", name))
			etag, err = app.createFunction(ctx, name, code)
			if err != nil {
				return "", fmt.Errorf("failed to create function, %w", err)
			}
		} else {
			return "", fmt.Errorf("function %s not found. To create a new function, add --create-if-missing flag", name)
		}
	} else {
		slog.Info(f("function %s found", name))
		functionConfig = res.FunctionSummary.FunctionConfig
		updateFunctionConfig := false
		if aws.ToString(functionConfig.Comment) != app.config.Comment {
			functionConfig.Comment = aws.String(app.config.Comment)
			updateFunctionConfig = true
		}
		if functionConfig.Runtime != app.config.Runtime {
			functionConfig.Runtime = app.config.Runtime
			updateFunctionConfig = true
		}
		associated, err := app.associateKVS(ctx, functionConfig)
		if err != nil {
			return "", fmt.Errorf("failed to associate kvs, %w", err)
		}

		if res, err := app.cloudfront.GetFunction(ctx, &cloudfront.GetFunctionInput{
			Name:  aws.String(name),
			Stage: Stage,
		}); err != nil {
			return "", fmt.Errorf("failed to describe function, %w", err)
		} else {
			etag = aws.ToString(res.ETag)
			if !bytes.Equal(res.FunctionCode, code) || updateFunctionConfig {
				slog.Info("function is changed, updating...")
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
			} else if associated {
				slog.Info("function and kvs associated is not changed")
			} else {
				slog.Info("function is not changed")
			}
		}
	}
	return etag, nil
}

func (app *CFFT) associateKVS(ctx context.Context, fc *types.FunctionConfig) (bool, error) {
	var associated bool
	if app.cfkvsArn == "" {
		// no kvs
		return false, nil
	}
	slog.Info(f("kvsArn: %s", app.cfkvsArn))
	if fc.KeyValueStoreAssociations != nil {
		for _, association := range fc.KeyValueStoreAssociations.Items {
			slog.Info(f("associated kvs: %s", aws.ToString(association.KeyValueStoreARN)))
			if aws.ToString(association.KeyValueStoreARN) == app.cfkvsArn {
				associated = true
			}
		}
	} else {
		fc.KeyValueStoreAssociations = &types.KeyValueStoreAssociations{
			Quantity: aws.Int32(0),
			Items:    []types.KeyValueStoreAssociation{},
		}
	}
	if !associated {
		slog.Info(f("associating kvs %s to function %s...", app.config.KVS.Name, app.config.Name))
		fc.KeyValueStoreAssociations.Items =
			append(
				fc.KeyValueStoreAssociations.Items,
				types.KeyValueStoreAssociation{
					KeyValueStoreARN: aws.String(app.cfkvsArn),
				},
			)
		fc.KeyValueStoreAssociations.Quantity = aws.Int32(int32(len(fc.KeyValueStoreAssociations.Items)))
	}
	return associated, nil
}

func (app *CFFT) runTestCase(ctx context.Context, name, etag string, c *TestCase) error {
	logger := slog.With("testcase", c.Identifier())
	logger.Info("testing function", "etag", etag)
	logger.Debug(string(c.EventBytes()))

	// retry policy for returning 0 ComputeUtilization
	policy := retry.Policy{
		MinDelay: 100 * time.Millisecond,
		MaxDelay: 1 * time.Second,
		MaxCount: 5,
	}
	var testResult *types.TestResult
	err := policy.Do(ctx, func() error {
		res, err := app.cloudfront.TestFunction(ctx, &cloudfront.TestFunctionInput{
			Name:        aws.String(name),
			IfMatch:     aws.String(etag),
			Stage:       Stage,
			EventObject: c.EventBytes(),
		})
		if err != nil {
			return retry.MarkPermanent(err)
		}
		testResult = res.TestResult

		if errMsg := aws.ToString(testResult.FunctionErrorMessage); errMsg != "" {
			return retry.MarkPermanent(errors.New(errMsg))
		}
		if testResult.ComputeUtilization == nil || *testResult.ComputeUtilization == "" || *testResult.ComputeUtilization == "0" {
			logger.Debug("ComputeUtilization: 0, retring...")
			return fmt.Errorf("ComputeUtilization: 0")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to test function, %w", err)
	}

	cu, err := strconv.Atoi(aws.ToString(testResult.ComputeUtilization))
	if err != nil {
		return fmt.Errorf("failed to parse compute utilization, %w", err)
	}
	switch {
	case 71 <= cu:
		logger.Warn(f("ComputeUtilization: %d very close to or exceeds the maximum allowed time", cu))
	case 51 <= cu:
		logger.Warn(f("ComputeUtilization: %d nearing the maximum allowed time", cu))
	default:
		logger.Info(f("ComputeUtilization: %d optimal", cu))
	}

	for _, l := range testResult.FunctionExecutionLogs {
		logger.Info(l, "from", name)
	}
	out := *testResult.FunctionOutput
	logger.Info("TestFunction API succeeded")
	logger.Debug(f("function output: %s", out))
	if c.expect == nil {
		logger.Info("no expected value. skipping checking function output")
		return nil
	}

	logger.Info("checking function output with expected value")
	result := &CFFExpect{}
	if err := json.Unmarshal([]byte(*testResult.FunctionOutput), result); err != nil {
		return fmt.Errorf("failed to parse function output, %w", err)
	}
	var options []jsondiff.Option
	if c.ignore != nil {
		options = append(options, jsondiff.Ignore(c.ignore))
	}
	diff, err := jsondiff.Diff(
		&jsondiff.Input{Name: "expect", X: c.expect.ToMap()},
		&jsondiff.Input{Name: "actual", X: result.ToMap()},
		options...,
	)
	if err != nil {
		return fmt.Errorf("failed to diff, %w", err)
	}
	if diff != "" {
		fmt.Print(coloredDiff(diff))
		return fmt.Errorf("expect and actual are not equal")
	} else {
		logger.Info("expect and actual are equal")
	}
	return nil
}
