package cfft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	event  []byte
	expect any
	ignore *gojq.Query
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
	c.event = eventBytes

	if len(c.event) == 0 {
		return errors.New("event is empty")
	}

	if len(c.Expect) > 0 {
		// expect is optional
		expectBytes, err := readFile(c.Expect)
		if err != nil {
			return fmt.Errorf("failed to read expect object, %w", err)
		}
		if err := json.Unmarshal(expectBytes, &c.expect); err != nil {
			return fmt.Errorf("failed to parse expect object, %w", err)
		}
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
		log.Fatalf("cannot set environment variable: %v", err)
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
