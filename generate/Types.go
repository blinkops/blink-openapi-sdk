package gen

import sdkPlugin "github.com/blinkops/blink-sdk/plugin"

const (
	Action = `{{range $Action := .}}
  {{$Action.Name }}:
    alias: {{ $Action.Alias }}
    parameters:{{ range $param := .Parameters}}
      {{ if badPrefix $param.Name }}"{{$param.Name}}":
      {{- else }}{{$param.Name}}:{{end}}
        alias: "{{ paramName $param.Alias }}"
        {{- if $param.Required }}
        required: true{{end}}
		{{- if $param.Default }}
        default: {{$param.Default}}{{end}}
		{{- if $param.Description }}
        description: "{{$param.Description}}"{{end}}
		{{- if $param.Format}} 
        type: {{ fixType $param.Format }}{{end}}
        index: {{ index $Action.Name }}{{ end}}{{ end}}`

	YAMLTemplate = `actions:` + Action

	READMETemplate = `## blink-{{ .Name }}
> {{ .Description }}
{{range .Actions}}
` + READMEAction + `
{{ end}}`

	READMEAction = `
## {{.Alias }}
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
    <tr>{{ range $param := .Parameters}}
       <tr>
           <td>{{ $param.Alias }}</td>
           <td>{{ $param.Description }}</td>
       </tr>{{ end}}
    </tr>
  </tbody>
</table>
`
	README       = "README.md"
	actionSuffix = ".action.yaml"
)

type GeneratedParameter struct {
	Name        string
	Alias       string
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Placeholder string   `yaml:"placeholder"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	Pattern     string   `yaml:"pattern"`  // optional: regex to validate in case of input component
	Options     []string `yaml:"options"`  // optional: the option list in case of dropdown\checkbox
	Index       int64    `yaml:"index"`    // optional: the ordinal number of the parameter in the parameter list
	Format      string   `yaml:"format"`   // optional: format of the field for example -> type: date, format: date_epoch
	IsMulti     bool     `yaml:"is_multi"` // optional: is this a multi-select field
}

type GeneratedAction struct {
	Alias       string
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"`
	Enabled     bool                 `yaml:"enabled"`
	EntryPoint  string               `yaml:"entry_point"`
	Parameters  []GeneratedParameter `yaml:"parameters"`
}

type GeneratedReadme struct {
	Name        string
	Description string
	Actions     []GeneratedAction
}

func newGeneratedParameter(a map[string]sdkPlugin.ActionParameter) []GeneratedParameter {
	generatedParameters := []GeneratedParameter{}

	for name, param := range a {
		generatedParameters = append(generatedParameters, GeneratedParameter{
			Name:        name,
			Alias:       genAlias(name),
			Type:        param.Type,
			Description: param.Description,
			Placeholder: param.Placeholder,
			Required:    param.Required,
			Default:     param.Default,
			Pattern:     param.Pattern,
			Options:     param.Options,
			Index:       param.Index,
			Format:      param.Format,
			IsMulti:     param.IsMulti,
		})
	}

	return generatedParameters
}

func newGeneratedAction(act sdkPlugin.Action) GeneratedAction {
	return GeneratedAction{
		Alias:       genAlias(act.Name),
		Name:        act.Name,
		Description: act.Description,
		Enabled:     act.Enabled,
		EntryPoint:  act.EntryPoint,
		Parameters:  newGeneratedParameter(act.Parameters),
	}
}
