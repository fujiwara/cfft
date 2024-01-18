package cfft

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/google/go-jsonnet"
	goconfig "github.com/kayac/go-config"
)

type Config struct {
	Name      string               `json:"name" yaml:"name"`
	Function  string               `json:"function" yaml:"function"`
	KVS       *KeyValueStoreConfig `json:"kvs" yaml:"kvs"`
	TestCases []*TestCase          `json:"testCases" yaml:"testCases"`

	functionCode []byte
	dir          string
	loader       *goconfig.Loader
}

// ReadFile supports jsonnet and yaml files. If the file is jsonnet or yaml, it will be evaluated and converted to json.
func ReadFile(p string) ([]byte, error) {
	b, err := goconfig.ReadWithEnv(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s, %w", p, err)
	}
	switch filepath.Ext(p) {
	case ".json", ".jsonnet":
		vm := jsonnet.MakeVM()
		s, err := vm.EvaluateAnonymousSnippet(p, string(b))
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate jsonnet %s, %w", p, err)
		}
		return []byte(s), nil
	case ".yaml", ".yml":
		var v any
		if err := yaml.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("failed to parse yaml %s, %w", p, err)
		}
		return json.Marshal(v)
	}
	// otherwise, return as is
	return b, nil
}

// ReadFile reads file from the same directory as config file.
func (c *Config) ReadFile(p string) ([]byte, error) {
	return ReadFile(filepath.Join(c.dir, p))
}

func (c *Config) FunctionCode() ([]byte, error) {
	if c.functionCode != nil {
		return c.functionCode, nil
	}
	b, err := c.ReadFile(c.Function)
	if err != nil {
		return nil, fmt.Errorf("failed to read function file %s, %w", c.Function, err)
	}
	c.functionCode = b
	return b, nil
}

func LoadConfig(ctx context.Context, path string) (*Config, error) {
	config := &Config{
		loader: goconfig.New(),
	}
	b, err := ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s, %w", path, err)
	}
	if err := json.Unmarshal(b, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s, %w", path, err)
	}
	config.dir = filepath.Dir(path)

	if config.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if config.Function == "" {
		return nil, fmt.Errorf("function is required")
	}

	for i, tc := range config.TestCases {
		tc.id = i
		if err := tc.Setup(ctx, config.ReadFile); err != nil {
			return nil, fmt.Errorf("failed to setup config %s, %w", tc.Name, err)
		}
	}
	return config, nil
}

type KeyValueStoreConfig struct {
	Name string `json:"name" yaml:"name"`
}
