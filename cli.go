package cfft

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/itchyny/gojq"
)

type CLI struct {
	Name     string `arg:"" help:"function name"`
	Function string `arg:"" help:"function code file"`
	Event    string `arg:"" help:"event object file"`
	Expect   string `arg:"" help:"expect object file" optional:"true"`
	Ignore   string `short:"i" long:"ignore" help:"ignore fields in the expect object by jq syntax"`
}

func (c *CLI) NewTestCase(ctx context.Context) (*TestCase, error) {
	testCase := TestCase{
		EventFile:  c.Event,
		ExpectFile: c.Expect,
		IgnoreStr:  c.Ignore,
	}
	log.Printf("[debug] NewTestCase: %#v", testCase)

	event, err := os.ReadFile(c.Event)
	if err != nil {
		return nil, fmt.Errorf("failed to read event object, %w", err)
	}
	var eventObject any
	if err := json.Unmarshal(event, &eventObject); err != nil {
		return nil, fmt.Errorf("failed to parse event object %s, %w", c.Event, err)
	}
	testCase.event = event

	if c.Expect != "" {
		expectBytes, err := os.ReadFile(c.Expect)
		if err != nil {
			return nil, fmt.Errorf("failed to read expect object, %w", err)
		}
		if err := json.Unmarshal(expectBytes, &testCase.expect); err != nil {
			return nil, fmt.Errorf("failed to parse expect object, %w", err)
		}
	}

	if c.Ignore != "" {
		q, err := gojq.Parse(c.Ignore)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ignore query, %w", err)
		}
		testCase.ignore = q
	}

	return &testCase, nil
}
