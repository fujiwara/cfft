package cfft

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

type TFDataCmd struct {
}

type TFDataOutout struct {
	Name    string                `json:"name"`
	Code    string                `json:"code"`
	Comment string                `json:"comment,omitempty"`
	Runtime types.FunctionRuntime `json:"runtime"`
	KVSArn  string                `json:"kvs_arn,omitempty"`
}

func (app *CFFT) RunTFData(ctx context.Context, opt *TFDataCmd) error {
	localCode, err := app.config.FunctionCode()
	if err != nil {
		return fmt.Errorf("failed to read function code, %w", err)
	}
	out := TFDataOutout{
		Name:    app.config.Name,
		Code:    string(localCode),
		Comment: app.config.Comment,
		Runtime: app.config.Runtime,
		KVSArn:  app.cfkvsArn,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
