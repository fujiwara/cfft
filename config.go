package cfft

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/goccy/go-yaml"
	"github.com/google/go-jsonnet"
	goconfig "github.com/kayac/go-config"
)

const ChainTemplate = `
const %s = async function (event) {
    %s
    return handler(event);
}
`

const MainTemplateRequest = `
%s

async function handler(event) {
	const fns = [%s];
	for (let i = 0; i < fns.length; i++) {
		const res = await fns[i](event);
		if (res && res.statusCode) {
			// when viewer-request returns response object, return it immediately
			return res;
		}
		event.request = res;
	}
	return event.request;
}
`

const MainTemplateResponse = `
%s

async function handler(event) {
	const fns = [%s];
	for (let i = 0; i < fns.length; i++) {
		event.response = await fns[i](event);
	}
	return event.response;
}
`

type Config struct {
	Name      string                `json:"name" yaml:"name"`
	Comment   string                `json:"comment" yaml:"comment"`
	Function  string                `json:"function" yaml:"function,omitempty"`
	Chain     *ConfigChain          `json:"chain,omitempty" yaml:"chain,omitempty"`
	Runtime   types.FunctionRuntime `json:"runtime" yaml:"runtime"`
	KVS       *KeyValueStoreConfig  `json:"kvs,omitempty" yaml:"kvs,omitempty"`
	TestCases []*TestCase           `json:"testCases" yaml:"testCases"`

	functionCode []byte
	dir          string
	path         string
	loader       *goconfig.Loader
}

type ConfigChain struct {
	EventType string   `json:"event_type" yaml:"event_type"`
	Functions []string `json:"functions" yaml:"functions"`
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
	if c.Function != "" {
		b, err := c.ReadFile(c.Function)
		if err != nil {
			return nil, fmt.Errorf("failed to read function file %s, %w", c.Function, err)
		}
		c.functionCode = b
		return b, nil
	} else if c.Chain != nil {
		// chain
		funcNames := make([]string, 0, len(c.Chain.Functions))
		codes := make([]string, 0, len(c.Chain.Functions))
		var imports []string
		for _, cf := range c.Chain.Functions {
			b, err := c.ReadFile(cf)
			if err != nil {
				return nil, fmt.Errorf("failed to read chain function file %s, %w", cf, err)
			}
			name := fmt.Sprintf("__chain_%x", md5.Sum(b))
			imp, code := grepImports(string(b))
			codes = append(codes, fmt.Sprintf(ChainTemplate, name, code))
			imports = append(imports, imp)
			funcNames = append(funcNames, name)
		}
		var main string
		switch c.Chain.EventType {
		case "viewer-request":
			main = fmt.Sprintf(MainTemplateRequest, strings.Join(imports, "\n"), strings.Join(funcNames, ", "))
		case "viewer-response":
			main = fmt.Sprintf(MainTemplateResponse, strings.Join(imports, "\n"), strings.Join(funcNames, ", "))
		default:
			return nil, fmt.Errorf("invalid chain event_type %s", c.Chain.EventType)
		}
		c.functionCode = []byte(strings.Join(codes, "\n") + main)
		return c.functionCode, nil
	} else {
		return nil, fmt.Errorf("function or chain_functions is required")
	}
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
	if config.Function == "" && config.Chain == nil {
		return nil, fmt.Errorf("function or chain is required")
	}
	if config.Chain != nil && config.Runtime != types.FunctionRuntimeCloudfrontJs20 {
		return nil, fmt.Errorf("chain is only supported for runtime %s", types.FunctionRuntimeCloudfrontJs20)
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

func grepImports(s string) (string, string) {
	var imports []string
	var code []string
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "import ") {
			imports = append(imports, line)
		} else {
			code = append(code, line)
		}
	}
	return strings.Join(imports, "\n"), strings.Join(code, "\n")
}
