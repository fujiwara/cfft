package cfft_test

import (
	"context"
	"testing"

	"github.com/fujiwara/cfft"
)

func TestSetup(t *testing.T) {
	ctx := context.Background()

	for _, ext := range []string{".json", ".jsonnet", ".yaml", ".yml"} {
		t.Run(ext, func(t *testing.T) {
			testCase := &cfft.TestCase{
				Event:  "testdata/event" + ext,
				Expect: "testdata/expect" + ext,
				Ignore: ".foo",
			}
			err := testCase.Setup(ctx, cfft.ReadFile)
			if err != nil {
				t.Errorf("Setup returned an error: %v", err)
			}
			ev := testCase.GetEvent()
			if ev.Version != "1.0" {
				t.Errorf("GetEvent returned unexpected version: %v", ev.Version)
			}
		})
	}
}

func TestSetupText(t *testing.T) {
	ctx := context.Background()

	for _, ext := range []string{".jsonnet", ".yaml"} {
		t.Run(ext, func(t *testing.T) {
			testCase := &cfft.TestCase{
				Event:  "testdata/text_event" + ext,
				Expect: "testdata/expect" + ext,
				Ignore: ".foo",
			}
			err := testCase.Setup(ctx, cfft.ReadFile)
			if err != nil {
				t.Errorf("Setup returned an error: %v", err)
			}
		})
	}
}
