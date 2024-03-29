package cfft

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/goccy/go-yaml"
	"github.com/google/go-jsonnet/formatter"
)

//go:embed defaults/viewer_request.js
var DefaultFunctionCodeViewerRequest []byte

//go:embed defaults/viewer_response.js
var DefaultFunctionCodeViewerResponse []byte

//go:embed defaults/viewer_request_event.json
var DefaultEventViewerRequest []byte

//go:embed defaults/viewer_response_event.json
var DefaultEventViewerResponse []byte

func DefaultFunctionCode(t string) []byte {
	switch t {
	case "viewer-request":
		return DefaultFunctionCodeViewerRequest
	case "viewer-response":
		return DefaultFunctionCodeViewerResponse
	default:
		panic(fmt.Sprintf("invalid event type %s", t))
	}
}

func DefaultEvent(t string) []byte {
	switch t {
	case "viewer-request":
		return DefaultEventViewerRequest
	case "viewer-response":
		return DefaultEventViewerResponse
	default:
		panic(fmt.Sprintf("invalid event type %s", t))
	}
}

type InitCmd struct {
	Name      string `help:"function name" required:"true"`
	Format    string `help:"output config and event file format (json,jsonnet,yaml)" default:"" enum:"jsonnet,json,yaml,yml,"`
	EventType string `help:"event type (viewer-request,viewer-response)" default:"viewer-request" enum:"viewer-request,viewer-response"`
}

func (app *CFFT) InitFunction(ctx context.Context, opt *InitCmd) error {
	name := opt.Name

	var code []byte
	var comment string
	runtime := DefaultRuntime
	var kvsConfig *KeyValueStoreConfig
	res, err := app.cloudfront.GetFunction(ctx, &cloudfront.GetFunctionInput{
		Name:  aws.String(name),
		Stage: Stage,
	})
	if err != nil {
		var notFound *types.NoSuchFunctionExists
		if !errors.As(err, &notFound) {
			return fmt.Errorf("failed to get function, %w", err)
		}
		slog.Info(f("function %s not found. using default code for %s", name, opt.EventType))
		code = DefaultFunctionCode(opt.EventType)
	} else {
		slog.Info(f("function %s found", name))
		code = res.FunctionCode

		slog.Info("detecting kvs association...")
		res, err := app.cloudfront.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
			Name:  aws.String(name),
			Stage: Stage,
		})
		if err != nil {
			return fmt.Errorf("failed to describe function, %w", err)
		}
		fnConf := res.FunctionSummary.FunctionConfig
		comment = aws.ToString(fnConf.Comment)
		runtime = fnConf.Runtime
		if kvsass := fnConf.KeyValueStoreAssociations; kvsass == nil {
			slog.Info(f("function %s has no kvs association", name))
		} else {
			for _, item := range kvsass.Items {
				if kvsConfig != nil {
					slog.Warn(f("function %s has multiple kvs associations. using %s", name, kvsConfig.Name))
					break
				}
				list, err := app.cloudfront.ListKeyValueStores(ctx, &cloudfront.ListKeyValueStoresInput{})
				if err != nil {
					return fmt.Errorf("failed to list kvs, %w", err)
				}
				for _, kvs := range list.KeyValueStoreList.Items {
					if aws.ToString(item.KeyValueStoreARN) == aws.ToString(kvs.ARN) {
						slog.Info(f("function %s is associated with kvs %s", name, *kvs.Name))
						kvsConfig = &KeyValueStoreConfig{
							Name: aws.ToString(kvs.Name),
						}
						break
					}
				}
			}
		}
	}

	// create function file
	slog.Info("creating function file: function.js")
	if err := WriteFile("function.js", code, 0644); err != nil {
		return fmt.Errorf("failed to write file, %w", err)
	}

	configFormat := opt.Format
	if configFormat == "" {
		configFormat = "yaml"
	}
	eventFormat := opt.Format
	if eventFormat == "" {
		eventFormat = "json"
	}

	// create config file
	config := &Config{
		Name:     name,
		Comment:  comment,
		Runtime:  runtime,
		Function: json.RawMessage(`"function.js"`),
		KVS:      kvsConfig,
		TestCases: []*TestCase{
			{
				Name:  "default",
				Event: "event." + eventFormat,
			},
		},
	}
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json, %w", err)
	}
	switch configFormat {
	case "jsonnet":
		slog.Info("creating config file: cfft.jsonnet")
		out, err := formatter.Format("cfft.jsonnet", string(configBytes), formatter.DefaultOptions())
		if err != nil {
			return fmt.Errorf("failed to format jsonnet, %w", err)
		}
		if err := WriteFile("cfft.jsonnet", []byte(out), 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	case "json":
		slog.Info("creating config file: cfft.json")
		if err := WriteFile("cfft.json", configBytes, 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	case "yaml", "yml":
		slog.Info("creating config file: cfft.yaml")
		yb, err := yaml.JSONToYAML(configBytes)
		if err != nil {
			return fmt.Errorf("failed to convert json to yaml, %w", err)
		}
		if err := WriteFile("cfft.yaml", yb, 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	default:
		return fmt.Errorf("invalid format %s", opt.Format)
	}

	// create event file
	slog.Info(f("creating event file event.%s", opt.Format))
	switch eventFormat {
	case "jsonnet":
		out, err := formatter.Format("event.jsonnet", string(DefaultEvent(opt.EventType)), formatter.DefaultOptions())
		if err != nil {
			return fmt.Errorf("failed to format jsonnet, %w", err)
		}
		if err := WriteFile("event.jsonnet", []byte(out), 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	case "json":
		if err := WriteFile("event.json", DefaultEvent(opt.EventType), 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	case "yaml", "yml":
		var event any
		if err := json.Unmarshal(DefaultEvent(opt.EventType), event); err != nil {
			return fmt.Errorf("failed to unmarshal json, %w", err)
		}
		b, err := yaml.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal yaml, %w", err)
		}
		if err := WriteFile("event."+opt.Format, b, 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	default:
		return fmt.Errorf("invalid format %s", opt.Format)
	}

	slog.Info("done")
	return nil
}

func WriteFile(path string, b []byte, perm fs.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("file %s already exists. overwrite? [y/N] ", path)
		var yesno string
		if _, err := fmt.Scanln(&yesno); err != nil {
			return nil
		}
		if yesno != "y" {
			return nil
		}
	}
	return os.WriteFile(path, b, perm)
}
