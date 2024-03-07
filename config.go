package cfft

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/goccy/go-yaml"
	"github.com/google/go-jsonnet"
	goconfig "github.com/kayac/go-config"
)

type Config struct {
	Name      string                `json:"name" yaml:"name"`
	Comment   string                `json:"comment" yaml:"comment"`
	Function  json.RawMessage       `json:"function" yaml:"function,omitempty"`
	Runtime   types.FunctionRuntime `json:"runtime" yaml:"runtime"`
	KVS       *KeyValueStoreConfig  `json:"kvs,omitempty" yaml:"kvs,omitempty"`
	TestCases []*TestCase           `json:"testCases" yaml:"testCases"`

	function     ConfigFunction
	functionCode []byte
	dir          string
	path         string
	loader       *goconfig.Loader
}

// ReadFile supports jsonnet and yaml files. If the file is jsonnet or yaml, it will be evaluated and converted to json.
func ReadFile(p string) ([]byte, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s, %w", p, err)
	}
	switch filepath.Ext(p) {
	case ".json", ".jsonnet":
		vm := jsonnet.MakeVM()
		return func() ([]byte, error) {
			// change directory to the file's directory
			// to resolve relative paths in jsonnet
			cd, _ := os.Getwd()
			defer os.Chdir(cd)
			d, f := filepath.Dir(p), filepath.Base(p)
			os.Chdir(d)
			s, err := vm.EvaluateAnonymousSnippet(f, string(b))
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate jsonnet %s, %w", p, err)
			}
			return goconfig.ReadWithEnvBytes([]byte(s))
		}()
	case ".yaml", ".yml":
		var v any
		if err := yaml.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("failed to parse yaml %s, %w", p, err)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to convert yaml to json %s, %w", p, err)
		}
		return goconfig.ReadWithEnvBytes(b)
	}
	// otherwise, return as is
	return goconfig.ReadWithEnvBytes(b)
}

// ReadFile reads file from the same directory as config file.
func (c *Config) ReadFile(p string) ([]byte, error) {
	return ReadFile(filepath.Join(c.dir, p))
}

func (c *Config) FunctionCode(ctx context.Context, hashCode []byte) ([]byte, error) {
	if c.functionCode != nil {
		return c.functionCode, nil
	}
	code, err := c.function.FunctionCode(ctx, c.ReadFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read function code, %w", err)
	}
	// hash function code
	h := sha256.New()
	if hashCode == nil {
		h.Write(code)
	} else {
		h.Write(hashCode)
	}
	sum := base64.StdEncoding.EncodeToString(h.Sum(nil))
	// append header comment
	buf := new(bytes.Buffer)
	buf.Grow(len(code) + 64)
	buf.WriteString("//cfft:" + sum + "\n")
	buf.Write(code)

	c.functionCode = buf.Bytes()
	slog.Debug(f("function code size: %d", len(c.functionCode)))
	if s := len(c.functionCode); s > MaxCodeSize {
		return nil, fmt.Errorf("function code size %d exceeds %d bytes", s, MaxCodeSize)
	}
	return c.functionCode, nil
}

func LoadConfig(ctx context.Context, path string) (*Config, error) {
	config := &Config{
		loader: goconfig.New(),
		path:   path,
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

	if bytes.HasPrefix(config.Function, []byte(`"`)) {
		// maybe string
		var s string
		if err := json.Unmarshal(config.Function, &s); err == nil {
			config.function.Functions = []string{s}
		}
	} else if err := json.Unmarshal(config.Function, &config.function); err != nil {
		return nil, fmt.Errorf("failed to parse function, %w", err)
	}

	if len(config.function.Functions) == 0 {
		return nil, fmt.Errorf("function is required")
	}
	if len(config.function.Functions) > 1 && config.Runtime != types.FunctionRuntimeCloudfrontJs20 {
		return nil, fmt.Errorf("chain functions feature is only supported for runtime %s", types.FunctionRuntimeCloudfrontJs20)
	}

	// validate runtime
	switch config.Runtime {
	case "":
		config.Runtime = DefaultRuntime
	case types.FunctionRuntimeCloudfrontJs10:
		if config.KVS != nil {
			return nil, fmt.Errorf("kvs is not supported for runtime %s", config.Runtime)
		}
	case types.FunctionRuntimeCloudfrontJs20: // == Default
		// ok
	default:
		return nil, fmt.Errorf("invalid runtime %s", config.Runtime)
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
