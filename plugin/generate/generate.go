package generate

import (
	"github.com/blinkops/blink-openapi-sdk/plugin"
	"github.com/urfave/cli/v2"
	"html/template"
	"os"
	"strings"
)

const (
	YAMLTemplate = `actions:{{range $action := .GetActions}}
  {{$action.Name }}:
    alias: {{ actName $action.Name }}
    parameters:{{ range $name, $param := .Parameters}}
      {{ if badPrefix $name }}"{{$name}}":
      {{- else }}{{$name}}:{{end}}
        alias: "{{ paramName $name }}"
        {{- if $param.Required }}
        required: true{{end}}
        {{- if $param.Format }}
        type: {{ $param.Format }}{{end}}
        index: {{ index $action.Name }}{{ end}}
{{ end}}`

	READMETemplate = `## blink-{{ .Describe.Name }}
> {{ .Describe.Description }}
{{range .GetActions}}
## {{.Name }}
* {{.Description }}
<table>
<caption>Action Parameters</caption>
  <thead>
    <tr>
        <th>Param Name</th>
        <th>Param Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>{{ range $name, $param := .Parameters}}
       <tr>
           <td>{{ $name }}</td>
           <td>{{ $param.Description }}</td>
       </tr>{{ end}}
    </tr>
  </tbody>
</table>
{{ end}}`
	README = "README.md"
)

func GenerateMaskFile(c *cli.Context) error {
	apiPlugin, err := plugin.NewOpenApiPlugin(nil, plugin.PluginMetadata{
		Name:        "",
		MaskFile:    "",
		OpenApiFile: c.String("file"),
	}, plugin.Callbacks{})

	if err != nil {
		return err
	}

	indexMap := map[string]int{}

	genAlias := func(str string) string {
		uppercaseWords := []string{"url", "id", "ip", "ssl"}

		// replace _ with ' '

		str = strings.ReplaceAll(str, "_", " ")
		str = strings.ReplaceAll(str, ".", "_")
		// iter over words in the string
		for _, word := range strings.Split(str, " ") {

			// check if the word is in out list.
			if plugin.StringInSlice(word, uppercaseWords) {
				str = strings.ReplaceAll(str, word, strings.ToUpper(word))
			}
		}

		return strings.Join(strings.Fields(strings.Title(str)), " ")
	}

	err = runTemplate(c.String("output"), YAMLTemplate, apiPlugin, template.FuncMap{
		"actName": genAlias,
		"paramName": func(str string) string {
			a := strings.Split(genAlias(str), ".")
			return a[len(a)-1]
		},
		"badPrefix": func(str string) bool {
			return strings.HasPrefix(str, "@")
		},
		"index": func(str string) int {
			if _, ok := indexMap[str]; ok {
				indexMap[str] += 1
				return indexMap[str]
			}
			indexMap[str] = 1
			return 1
		},
	})
	if err != nil {
		return err
	}

	return nil

}

func GenerateMarkdown(c *cli.Context) error {

	apiPlugin, err := plugin.NewOpenApiPlugin(nil, plugin.PluginMetadata{
		Name:        c.String("name"),
		MaskFile:    c.String("mask"),
		OpenApiFile: c.String("file"),
	}, plugin.Callbacks{})

	if err != nil {
		return err
	}

	err = runTemplate(README, READMETemplate, apiPlugin, nil)
	if err != nil {
		return err
	}

	return nil

}

func runTemplate(fileName string, templateStr string, obj interface{}, funcs template.FuncMap) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	tmpl, err := template.New("").Funcs(funcs).Parse(templateStr)

	if err != nil {
		return err
	}

	if err := tmpl.Execute(f, obj); err != nil {
		return err
	}

	return nil
}
