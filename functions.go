package cfft

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/mattn/go-shellwords"
)

const MaxCodeSize = 10 * 1024

var FuncMap = template.FuncMap{
	"join": strings.Join,
}

type FuncTemplateArgs struct {
	Name string
	Code string
}

var FuncTemplate = template.Must(template.New("func").Parse(`
const {{.Name}} = async function(event) {
  {{.Code}}
  return handler(event);
}
`))

type MainTemplateArgs struct {
	Imports   []string
	FuncNames []string
	FuncCodes []string
}

var MainTemplateRequest = template.Must(template.New("main").Funcs(FuncMap).Parse(`
{{- range .Imports }}
{{.}}
{{- end -}}
{{- range .FuncCodes }}
{{.}}
{{- end -}}

async function handler(event) {
  const funcs = [{{ join .FuncNames ","}}];
  for (let i = 0; i < funcs.length; i++) {
    const res = await funcs[i](event);
    if (res && res.statusCode) {
      // when viewer-request returns response object, return it immediately
      return res;
    }
    event.request = res;
  }
  return event.request;
}
`))

var MainTemplateResponse = template.Must(template.New("main").Funcs(FuncMap).Parse(`
{{- range .Imports -}}
{{.}}
{{- end -}}

{{- range .FuncCodes -}}
{{.}}
{{- end -}}

async function handler(event) {
  const funcs = [{{ join .FuncNames "," }}];
  for (let i = 0; i < funcs.length; i++) {
    event.response = await funcs[i](event);
  }
  return event.response;
}
`))

type ConfigFunction struct {
	EventType     string   `json:"event_type" yaml:"event_type"`
	Functions     []string `json:"functions" yaml:"functions"`
	FilterCommand string   `json:"filter_command" yaml:"filter_command"`
}

func (c *ConfigFunction) FunctionCode(ctx context.Context, readFile func(string) ([]byte, error)) ([]byte, error) {
	if len(c.Functions) == 1 {
		// single function
		if b, err := readFile(c.Functions[0]); err != nil {
			return nil, fmt.Errorf("failed to read function file %s, %w", c.Functions[0], err)
		} else {
			return c.runFilter(ctx, b)
		}
	}
	// chain function
	slog.Debug("generating chain function code")
	funcNames := make([]string, 0, len(c.Functions))
	funcCodes := make([]string, 0, len(c.Functions))
	imports := make([]string, 0)
	for _, cf := range c.Functions {
		slog.Debug(f("reading chain function file %s", cf))
		b, err := readFile(cf)
		if err != nil {
			return nil, fmt.Errorf("failed to read chain function file %s, %w", cf, err)
		}
		name := fmt.Sprintf("__chain_%x", md5.Sum(b))
		imps, code := splitCode(string(b))
		buf := strings.Builder{}
		if err := FuncTemplate.Execute(&buf, FuncTemplateArgs{Name: name, Code: code}); err != nil {
			return nil, fmt.Errorf("failed to execute function template, %w", err)
		}
		funcCodes = append(funcCodes, buf.String())
		imports = append(imports, imps...)
		funcNames = append(funcNames, name)
	}

	var tmpl *template.Template
	switch c.EventType {
	case "viewer-request":
		tmpl = MainTemplateRequest
	case "viewer-response":
		tmpl = MainTemplateResponse
	default:
		return nil, fmt.Errorf("invalid chain event_type %s", c.EventType)
	}

	buf := bytes.Buffer{}
	if err := tmpl.Execute(&buf, MainTemplateArgs{Imports: imports, FuncNames: funcNames, FuncCodes: funcCodes}); err != nil {
		return nil, fmt.Errorf("failed to execute main template, %w", err)
	}
	return c.runFilter(ctx, buf.Bytes())
}

func splitCode(s string) ([]string, string) {
	importRegexp := regexp.MustCompile(`(?m)^import\s+.*?;$`)
	imports := importRegexp.FindAllString(s, -1)
	nonImports := importRegexp.ReplaceAllString(s, "")
	return imports, nonImports
}

func (c *ConfigFunction) runFilter(ctx context.Context, b []byte) ([]byte, error) {
	if c.FilterCommand == "" {
		return b, nil
	}
	slog.Info(f("running filter command %s", c.FilterCommand))
	cmds, err := shellwords.Parse(c.FilterCommand)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filter command, %w", err)
	}

	var cmd *exec.Cmd
	switch len(cmds) {
	case 0:
		return nil, fmt.Errorf("empty filter command")
	case 1:
		cmd = exec.CommandContext(ctx, cmds[0])
	default:
		cmd = exec.CommandContext(ctx, cmds[0], cmds[1:]...)
	}
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 60 * time.Second
	cmd.Stdin = bytes.NewReader(b)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run filter command, %w", err)
	}
	return out.Bytes(), nil
}
