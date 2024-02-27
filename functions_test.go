package cfft_test

import (
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
	code string
}

func (r *localRunner) Run(ctx context.Context, name, _ string, event []byte, logger *slog.Logger) ([]byte, error) {
	logger.Info(fmt.Sprintf("running function %s at local", name))
	chaindCode := r.code + "\n" + fmt.Sprintf(runHandler, string(event))
	logger.Info(chaindCode)
	cmd := exec.CommandContext(ctx, "node", "-e", chaindCode)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(string(out))
		return nil, err
	}
	return out, nil
}

func TestChainFunction(t *testing.T) {
	ctx := context.Background()
	conf, err := cfft.LoadConfig(ctx, path.Join("testdata/chain/cfft.yaml"))
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
	app.SetRunner(&localRunner{code: string(code)})
	for _, cs := range app.Config().TestCases {
		if err := app.RunTestCase(ctx, "chain", "", cs); err != nil {
			t.Errorf("failed to test: %v", err)
		}
	}
}
