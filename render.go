package cfft

import (
	"context"
	"fmt"
	"log/slog"
)

type RenderCmd struct {
	Target   string `arg:"" help:"render target (function,event,expect)" default:"function" enum:"function,event,expect"`
	TestCase string `cmd:"" help:"test case name (for target event or expect)" default:""`
}

func (app *CFFT) Render(ctx context.Context, opt *RenderCmd) error {
	switch opt.Target {
	case "function", "": // default target
		return app.renderFunction(ctx)
	case "event", "expect":
		return app.renderTestCase(ctx, opt)
	default:
		return fmt.Errorf("invalid target %s", opt.Target)
	}
}

func (app *CFFT) renderFunction(ctx context.Context) error {
	localCode, err := app.config.FunctionCode(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to read function code, %w", err)
	}
	if _, err := app.stdout.Write(localCode); err != nil {
		return fmt.Errorf("failed to write function code into STDOUT, %w", err)
	}
	return nil
}

func (app *CFFT) renderTestCase(ctx context.Context, opt *RenderCmd) error {
	for _, tc := range app.config.TestCases {
		// if test case name is empty, render first test case
		if tc.Name == opt.TestCase || opt.TestCase == "" {
			var b []byte
			switch opt.Target {
			case "event":
				b = tc.EventBytes()
			case "expect":
				b = tc.ExpectBytes()
			default:
				return fmt.Errorf("invalid target %s", opt.Target)
			}
			slog.Info(f("rendering %s of test case %s", opt.Target, tc.Name))
			if _, err := app.stdout.Write(b); err != nil {
				return fmt.Errorf("failed to write %s into STDOUT, %w", opt.Target, err)
			}
			return nil
		}
	}
	return fmt.Errorf("test case %s not found", opt.TestCase)
}
