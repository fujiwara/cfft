package cfft_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/fujiwara/cfft"
)

var TFResourceCases = []struct {
	Dir          string
	ResourceName string
}{
	{Dir: "funcv1", ResourceName: ""},
	{Dir: "funcv1", ResourceName: "named-v1"},
	{Dir: "funcv2", ResourceName: ""},
	{Dir: "funcv2", ResourceName: "named-v2"},
}

func TestTFResource(t *testing.T) {
	ctx := context.Background()
	for _, cs := range TFResourceCases {
		t.Run("tf-resource-"+cs.Dir, func(t *testing.T) {
			var rname string
			conf, err := cfft.LoadConfig(ctx, path.Join("testdata", cs.Dir, "/cfft.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if cs.ResourceName != "" {
				rname = cs.ResourceName
			} else {
				rname = conf.Name
			}
			app, err := cfft.New(ctx, conf)
			if err != nil {
				t.Fatal(err)
			}
			b := &bytes.Buffer{}
			app.SetStdout(b)
			publish := true
			opt := &cfft.TFCmd{
				External:     false,
				Publish:      &publish,
				ResourceName: cs.ResourceName,
			}
			if err := app.RunTF(ctx, opt); err != nil {
				t.Fatal(err)
			}
			var m map[string]any
			if err := json.Unmarshal(b.Bytes(), &m); err != nil {
				t.Log("failed to parse json", err)
			}
			code := m["resource"].(map[string]any)["aws_cloudfront_function"].(map[string]any)[rname].(map[string]any)["code"].(string)
			var rawCode string
			if strings.Contains(code, "${var.") {
				varName := strings.TrimSuffix(strings.TrimPrefix(code, "${var."), "}")
				t.Logf("varName: %s", varName)
				rawCode = m["variable"].(map[string]any)[varName].(map[string]any)["default"].(string)
			} else {
				rawCode = code
			}
			b.Reset()
			if err := app.Render(ctx, &cfft.RenderCmd{}); err != nil {
				t.Error("failed to render", err)
			}
			if !bytes.Equal(b.Bytes(), []byte(rawCode)) {
				t.Error("rendered code is not same as tf.json")
			}
		})
	}
}

func TestTFExternalData(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"funcv1", "funcv2"} {
		t.Run("tf-resource-"+name, func(t *testing.T) {
			conf, err := cfft.LoadConfig(ctx, path.Join("testdata", name, "/cfft.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			app, err := cfft.New(ctx, conf)
			if err != nil {
				t.Fatal(err)
			}
			b := &bytes.Buffer{}
			app.SetStdout(b)
			publish := true
			if err := app.RunTF(ctx, &cfft.TFCmd{External: true, Publish: &publish}); err != nil {
				t.Fatal(err)
			}
			var m map[string]string // for external data source, all values must be string
			if err := json.Unmarshal(b.Bytes(), &m); err != nil {
				t.Log("failed to parse json", err)
			}
			code, _ := os.ReadFile(path.Join("testdata", name, "function.js"))
			if m["code"] != string(code) {
				t.Error("external data source code is not same as function.js")
			}
			if m["name"] != conf.Name {
				t.Error("external data source name is not same as cfft.yaml")
			}
			if m["runtime"] != string(conf.Runtime) {
				t.Error("external data source runtime is not same as cfft.yaml")
			}
			if m["comment"] != string(conf.Comment) {
				t.Error("external data source comment is not same as cfft.yaml")
			}
		})
	}
}
