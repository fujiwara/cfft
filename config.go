package cfft

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/google/go-jsonnet"
)

type Config struct {
	Name      string      `json:"name" yaml:"name"`
	Function  string      `json:"function" yaml:"function"`
	TestCases []*TestCase `json:"testCases" yaml:"testCases"`

	functionCode []byte
	dir          string
}

// ReadFile supports jsonnet and yaml files. If the file is jsonnet or yaml, it will be evaluated and converted to json.
func ReadFile(p string) ([]byte, error) {
	switch filepath.Ext(p) {
	case ".json", ".jsonnet":
		vm := jsonnet.MakeVM()
		s, err := vm.EvaluateFile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate jsonnet %s, %w", p, err)
		}
		return []byte(s), nil
	case ".yaml", ".yml":
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s, %w", p, err)
		}
		var v any
		if err := yaml.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("failed to parse yaml %s, %w", p, err)
		}
		return json.Marshal(v)
	}
	// other files are treated as plain text
	return os.ReadFile(p)
}

// ReadFile reads file from the same directory as config file.
func (c *Config) ReadFile(p string) ([]byte, error) {
	return ReadFile(filepath.Join(c.dir, p))
}

func LoadConfig(ctx context.Context, path string) (*Config, error) {
	config := &Config{}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s, %w", path, err)
	}
	switch filepath.Ext(path) {
	case ".json":
		if err := json.Unmarshal(b, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s, %w", path, err)
		}
	case ".jsonnet":
		vm := jsonnet.MakeVM()
		s, err := vm.EvaluateFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate jsonnet %s, %w", path, err)
		}
		if err := json.Unmarshal([]byte(s), config); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s, %w", path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(b, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s, %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format %s", path)
	}
	config.dir = filepath.Dir(path)

	if config.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if config.Function == "" {
		return nil, fmt.Errorf("function is required")
	}
	b, err = config.ReadFile(config.Function)
	if err != nil {
		return nil, fmt.Errorf("failed to read function file %s, %w", config.Function, err)
	}
	config.functionCode = b
	for i, tc := range config.TestCases {
		tc.id = i
		if err := tc.Setup(ctx, config.ReadFile); err != nil {
			return nil, fmt.Errorf("failed to setup config %s, %w", tc.Name, err)
		}
	}
	return config, nil
}
