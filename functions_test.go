package cfft_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path"
	"testing"

	"github.com/fujiwara/cfft"
)

const runHandler = `
(async () => {
  const res = await handler(%s);
  console.log(JSON.stringify({ request: res }));
})()
`

type localRunner struct {
	code []byte
}

func (r *localRunner) Run(ctx context.Context, name, _ string, event []byte, logger *slog.Logger) ([]byte, error) {
	logger.Info(fmt.Sprintf("running function %s at local", name))
	code := r.code
	code = append(code, []byte(fmt.Sprintf(runHandler, string(event)))...)
	cmd := exec.CommandContext(ctx, "node")
	cmd.Stdin = bytes.NewReader(code)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(string(out))
		return nil, err
	}
	return out, nil
}

func TestLocalRunner(t *testing.T) {
	dirs := []string{"funcv1", "funcv2", "chain", "partial-event"}
	ctx := cfft.NewTestContext()
	for _, dir := range dirs {
		t.Run(dir, func(t *testing.T) {
			conf, err := cfft.LoadConfig(ctx, path.Join("testdata", dir, "cfft.yaml"))
			if err != nil {
				t.Errorf("failed to load config: %v", err)
			}
			app, err := cfft.New(ctx, conf)
			if err != nil {
				t.Error(err)
			}
			code, err := app.Config().FunctionCode(ctx)
			if err != nil {
				t.Error(err)
			}
			app.SetRunner(&localRunner{code: code})
			for _, cs := range app.Config().TestCases {
				if err := app.RunTestCase(ctx, "test-etag", cs); err != nil {
					t.Errorf("failed to test: %v", err)
				}
			}
		})
	}
}
