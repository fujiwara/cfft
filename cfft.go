package cfft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
		log.Printf("[info] kvs %s found", app.config.KVS.Name)
		app.envs["KVS_ID"] = aws.ToString(res.KeyValueStore.Id)
		app.envs["KVS_NAME"] = aws.ToString(res.KeyValueStore.Name)
		app.cfkvsArn = aws.ToString(res.KeyValueStore.ARN)
		return nil
	}
	// not found
	if !create {
		log.Printf("[warn] failed to describe kvs %s, %s", name, err)
		return fmt.Errorf("kvs %s not found. To create a new kvs, add --create-if-missing flag", name)
	}

	// create
	log.Printf("[info] kvs %s not found, creating...", name)
	if res, err := app.cloudfront.CreateKeyValueStore(ctx, &cloudfront.CreateKeyValueStoreInput{
		Name:    aws.String(name),
		Comment: aws.String("created by cfft"),
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
			log.Printf("[info] kvs %s created", name)
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
		log.Printf("[info] %s", err)
		return err
	}
}

func (app *CFFT) TestFunction(ctx context.Context, opt *TestCmd) error {
	if err := opt.Setup(); err != nil {
		return err
	}
	code, err := app.config.FunctionCode()
	if err != nil {
		return fmt.Errorf("failed to load function code, %w", err)
	}
	etag, err := app.prepareFunction(ctx, app.config.Name, code, opt.CreateIfMissing)
	if err != nil {
		return fmt.Errorf("failed to prepare function, %w", err)
	}

	for _, testCase := range app.config.TestCases {
		if !opt.ShouldRun(testCase.Identifier()) {
			log.Printf("[debug] skipping test case %s", testCase.Identifier())
			continue
		}
		if err := app.runTestCase(ctx, app.config.Name, etag, testCase); err != nil {
			return fmt.Errorf("failed to run test case %s, %w", testCase.Identifier(), err)
		}
	}
	return nil
}

func (app *CFFT) createFunction(ctx context.Context, name string, code []byte) (string, error) {
	log.Printf("[info] creating function %s...", name)
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
			Comment:                   aws.String("created by cfft"),
			Runtime:                   types.FunctionRuntimeCloudfrontJs20,
			KeyValueStoreAssociations: kvsassociation,
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
			if bytes.Equal(res.FunctionCode, code) {
				if associated {
					log.Println("[info] function code and kvs associated is not changed")
				} else {
					log.Println("[info] function code is not changed")
				}
			} else {
				log.Println("[info] function code or kvs association is changed, updating...")
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

func (app *CFFT) associateKVS(ctx context.Context, fc *types.FunctionConfig) (bool, error) {
	var associated bool
	if app.cfkvsArn == "" {
		// no kvs
		return false, nil
	}
	log.Println("[info] kvsArn:", app.cfkvsArn)
	if fc.KeyValueStoreAssociations != nil {
		for _, association := range fc.KeyValueStoreAssociations.Items {
			log.Println("[info] associated kvs:", aws.ToString(association.KeyValueStoreARN))
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
		log.Printf("[info] associating kvs %s to function %s...", app.config.KVS.Name, app.config.Name)
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
	log.Printf("[info] testing function %s with case %s...", name, c.Identifier())
	log.Printf("[debug] event: %s", string(c.EventBytes()))
	res, err := app.cloudfront.TestFunction(ctx, &cloudfront.TestFunctionInput{
		Name:        aws.String(name),
		IfMatch:     aws.String(etag),
		Stage:       Stage,
		EventObject: c.EventBytes(),
	})
	if err != nil {
		return fmt.Errorf("failed to test function, %w", err)
	}
	var failed bool
	if errMsg := aws.ToString(res.TestResult.FunctionErrorMessage); errMsg != "" {
		log.Printf("[error][%s] %s", c.Identifier(), errMsg)
		failed = true
	}
	log.Printf("[info][%s] ComputeUtilization:%s", c.Identifier(), aws.ToString(res.TestResult.ComputeUtilization))
	for _, l := range res.TestResult.FunctionExecutionLogs {
		log.Println(l)
	}
	out := *res.TestResult.FunctionOutput
	if failed {
		log.Printf("[info][%s] function output: %s", c.Identifier(), out)
		return errors.New("test failed")
	} else {
		log.Printf("[debug][%s] function output: %s", c.Identifier(), out)
	}
	if c.expect == nil {
		return nil
	}

	var result CFFExpect
	if err := json.Unmarshal([]byte(*res.TestResult.FunctionOutput), &result); err != nil {
		return fmt.Errorf("failed to parse function output, %w", err)
	}
	var options []jsondiff.Option
	if c.ignore != nil {
		options = append(options, jsondiff.Ignore(c.ignore))
	}
	diff, err := jsondiff.Diff(
		&jsondiff.Input{Name: "expect", X: c.expect},
		&jsondiff.Input{Name: "actual", X: result},
		options...,
	)
	if err != nil {
		return fmt.Errorf("failed to diff, %w", err)
	}
	if diff != "" {
		fmt.Print(coloredDiff(diff))
		return fmt.Errorf("expect and actual are not equal")
	} else {
		log.Printf("[info][%s] OK", c.Identifier())
	}
	return nil
}
