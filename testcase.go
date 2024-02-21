package cfft

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/itchyny/gojq"
)

type TestCase struct {
	Name   string            `json:"name" yaml:"name"`
	Event  string            `json:"event" yaml:"event"`
	Expect string            `json:"expect" yaml:"expect"`
	Ignore string            `json:"ignore" yaml:"ignore"`
	Env    map[string]string `json:"env" yaml:"env"`

	id     int
	event  *CFFEvent
	expect *CFFExpect
	ignore *gojq.Query
}

type CFFExpect struct {
	Request *CFFRequest  `json:"request,omitempty"`
	Reponse *CFFResponse `json:"response,omitempty"`
}

func (e *CFFExpect) ToMap() map[string]any {
	b, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		panic(err)
	}
	return m
}

func (c *TestCase) EventBytes() []byte {
	return c.event.Bytes()
}

func (c *TestCase) ExpectBytes() []byte {
	b, _ := json.Marshal(c.expect)
	return b
}

func (c *TestCase) Identifier() string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("[%d]", c.id)
}

func (c *TestCase) Setup(ctx context.Context, readFile func(string) ([]byte, error)) error {
	for k, v := range c.Env {
		df := localEnv(k, v)
		defer df()
	}

	eventBytes, err := readFile(c.Event)
	if err != nil {
		return fmt.Errorf("failed to read event object, %w", err)
	}
	var event CFFEvent
	if err := json.Unmarshal(eventBytes, &event); err != nil {
		return fmt.Errorf("failed to parse event object as CFF event object, %w", err)
	}
	c.event = &event

	if len(c.Expect) > 0 {
		// expect is optional
		expectBytes, err := readFile(c.Expect)
		if err != nil {
			return fmt.Errorf("failed to read expect object, %w", err)
		}
		if len(expectBytes) == 0 {
			return fmt.Errorf("expect object is empty")
		} else {
			slog.Debug(f("expect object: %s", string(expectBytes)))
		}
		var expect CFFExpect
		if err := json.Unmarshal(expectBytes, &expect); err != nil {
			return fmt.Errorf("failed to parse expect object, %w", err)
		}
		c.expect = &expect
	}

	if len(c.Ignore) > 0 {
		// ignore is optional
		q, err := gojq.Parse(c.Ignore)
		if err != nil {
			return fmt.Errorf("failed to parse ignore query, %w", err)
		}
		c.ignore = q
	}
	return nil
}

func localEnv(key, value string) func() {
	prevValue, ok := os.LookupEnv(key)

	if err := os.Setenv(key, value); err != nil {
		slog.Error("cannot set environment variable: %v", err)
		panic(err)
	}

	if ok {
		return func() {
			os.Setenv(key, prevValue)
		}
	} else {
		return func() {
			os.Unsetenv(key)
		}
	}
}
