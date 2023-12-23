package cfft

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

func (app *CFFT) InitFunction(ctx context.Context, opt InitCmd) error {
	name := opt.Name

	var code []byte
	res, err := app.cloudfront.GetFunction(ctx, &cloudfront.GetFunctionInput{
		Name:  aws.String(name),
		Stage: Stage,
	})
	if err != nil {
		var notFound types.EntityNotFound
		if errors.Is(err, &notFound) {
			return fmt.Errorf("failed to describe function, %w", err)
		}
		log.Printf("[info] function %s not found. using default code for %s", name, opt.EventType)
		code = DefaultFunctionCode(opt.EventType)
	} else {
		log.Printf("[info] function %s found", name)
		code = res.FunctionCode
	}

	// create function file
	log.Printf("[info] creating function file function.js")
	if err := os.WriteFile("function.js", code, 0644); err != nil {
		return fmt.Errorf("failed to write file, %w", err)
	}

	// create config file
	config := &Config{
		Name:     name,
		Function: "function.js",
		TestCases: []*TestCase{
			{
				Name:  "default",
				Event: "event." + opt.Format,
			},
		},
	}
	if b, err := yaml.Marshal(config); err != nil {
		return fmt.Errorf("failed to marshal yaml, %w", err)
	} else {
		log.Printf("[info] creating config file cfft.yaml")
		if err := os.WriteFile("cfft.yaml", b, 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	}

	// create event file
	log.Printf("[info] creating event file event.%s", opt.Format)
	switch opt.Format {
	case "jsonnet":
		out, err := formatter.Format("event.jsonnet", string(DefaultEvent(opt.EventType)), formatter.DefaultOptions())
		if err != nil {
			return fmt.Errorf("failed to format jsonnet, %w", err)
		}
		if err := os.WriteFile("event.jsonnet", []byte(out), 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	case "json":
		if err := os.WriteFile("event.json", DefaultEvent(opt.EventType), 0644); err != nil {
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
		if err := os.WriteFile("event."+opt.Format, b, 0644); err != nil {
			return fmt.Errorf("failed to write file, %w", err)
		}
	default:
		return fmt.Errorf("invalid format %s", opt.Format)
	}

	log.Println("[info] done")
	return nil
}
