package cfft

import (
	"context"
	"fmt"
	"os"
)

type RenderCmd struct {
}

func (app *CFFT) Render(ctx context.Context, opt *RenderCmd) error {
	localCode, err := app.config.FunctionCode()
	if err != nil {
		return fmt.Errorf("failed to read function code, %w", err)
	}
	if _, err := os.Stdout.Write(localCode); err != nil {
		return fmt.Errorf("failed to write function code into STDOUT, %w", err)
	}
	return nil
}
