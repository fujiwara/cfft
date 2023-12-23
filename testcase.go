package cfft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/itchyny/gojq"
)

type TestCase struct {
	Name   string `json:"name" yaml:"name"`
	Event  string `json:"event" yaml:"event"`
	Expect string `json:"expect" yaml:"expect"`
	Ignore string `json:"ignore" yaml:"ignore"`

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
