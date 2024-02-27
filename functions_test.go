package cfft_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/fujiwara/cfft"
	"github.com/google/go-cmp/cmp"
)

const runHandler = `
(async () => {
  const res = await handler(%s);
  console.log(JSON.stringify({ request: res }));
})()
`

func TestChainFunction(t *testing.T) {
	ctx := context.Background()
	conf, err := cfft.LoadConfig(ctx, path.Join("testdata/chain/cfft.yaml"))
	if err != nil {
		t.Errorf("failed to load config: %v", err)
	}
	app, err := cfft.New(ctx, conf)
	if err != nil {
		t.Errorf("failed to create app: %v", err)
	}
	b := &bytes.Buffer{}
	app.SetStdout(b)
	if err := app.Render(ctx, &cfft.RenderCmd{}); err != nil {
		t.Errorf("failed to render: %v", err)
	}
	event, err := os.ReadFile("testdata/chain/event.json")
	if err != nil {
		t.Errorf("failed to read event.json: %v", err)
	}
	chaindCode := b.String() + "\n" + fmt.Sprintf(runHandler, string(event))
	cmd := exec.CommandContext(ctx, "node", "-e", chaindCode)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("failed to run chaind code: %v", err)
	}
	t.Log(string(out))
	var result cfft.CFFExpect
	if err := json.Unmarshal(out, &result); err != nil {
		t.Errorf("failed to parse output: %v", err)
	}
	var expect cfft.CFFExpect
	if b, err := os.ReadFile("testdata/chain/expect.json"); err != nil {
		t.Errorf("failed to read expect.json: %v", err)
	} else {
		t.Log(string(b))
		if err := json.Unmarshal(b, &expect); err != nil {
			t.Errorf("failed to parse expect.json: %v", err)
		}
	}
	if diff := cmp.Diff(expect, result); diff != "" {
		t.Errorf("unexpected output: %v", diff)
	}
}
