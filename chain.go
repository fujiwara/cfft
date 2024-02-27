package cfft

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

var FuncMap = template.FuncMap{
	"join": strings.Join,
}

type FuncTemplateArgs struct {
	Name string
	Code string
}

var FuncTemplate = template.Must(template.New("func").Parse(`
const {{.Name}} = async (event) => {
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

const handler = async (event) => {
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

const handler = async (event) => {
  const funcs = [{{ join .FuncNames "," }}];
  for (let i = 0; i < funcs.length; i++) {
    event.response = await funcs[i](event);
  }
  return event.response;
}
`))

type ConfigChain struct {
	EventType string   `json:"event_type" yaml:"event_type"`
	Functions []string `json:"functions" yaml:"functions"`
}

func (c *ConfigChain) FunctionCode(readFile func(string) ([]byte, error)) ([]byte, error) {
	funcNames := make([]string, 0, len(c.Functions))
	funcCodes := make([]string, 0, len(c.Functions))
	imports := make([]string, 0)
	for _, cf := range c.Functions {
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
	return buf.Bytes(), nil
}

func splitCode(s string) ([]string, string) {
	importRegexp := regexp.MustCompile(`(?m)^import\s+.*?;$`)
	imports := importRegexp.FindAllString(s, -1)
	nonImports := importRegexp.ReplaceAllString(s, "")
	return imports, nonImports
}
