package gen

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-openapi-sdk/plugin"
	sdkPlugin "github.com/blinkops/blink-sdk/plugin"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	Action = `{{range $Action := .}}
  {{$Action.Name }}:
    alias: {{ genAlias $Action.Alias }}
    parameters:{{ range $name, $param := .Parameters}}
      {{ if badPrefix $name }}"{{$name}}":
      {{- else }}{{$name}}:{{end}}
        alias: "{{ paramName $param.Alias }}"
        {{- if $param.Required }}
        required: true{{end}}
		{{- if $param.Default }}
        default: {{$param.Default}}{{end}}
		{{- if $param.Description }}
        description: {{$param.Description}}{{end}}
		{{- if $param.Format}} 
        type: {{ fixType $param.Format }}{{end}}
        index: {{ index $Action.Name }}{{ end}}{{ end}}`

	YAMLTemplate = `actions:` + Action

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

func StringInSlice(name string, array []string) bool {
	for _, elm := range array {
		if elm == name {
			return true
		}
	}
	return false
}

type Parameter struct {
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

type EnhancedAction struct {
	Alias       string
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"`
	Enabled     bool                 `yaml:"enabled"`
	EntryPoint  string               `yaml:"entry_point"`
	Parameters  map[string]Parameter `yaml:"parameters"`
}

func newParameter(a map[string]sdkPlugin.ActionParameter) map[string]Parameter {
	newMap := map[string]Parameter{}

	for name, param := range a {
		newMap[name] = Parameter{
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
		}
	}

	return newMap
}

func newCliAction(act sdkPlugin.Action) EnhancedAction {
	return EnhancedAction{
		Alias:       genAlias(act.Name),
		Name:        act.Name,
		Description: act.Description,
		Enabled:     act.Enabled,
		EntryPoint:  act.EntryPoint,
		Parameters:  newParameter(act.Parameters),
	}
}

// FilterMaskedParameters returns a new ParameterName with the same parameters as the masked ParameterName.
func FilterMaskedParameters(maskedAct *mask.MaskedAction, act sdkPlugin.Action, filterParameters bool) EnhancedAction {
	newAction := newCliAction(act)

	if !filterParameters { // return the original action
		return newAction
	}

	newParameters := map[string]Parameter{}

	for parmName, mParam := range maskedAct.Parameters {
		for name, parameter := range act.Parameters {
			if name == parmName { // if the ParameterName name is also in the mask file.
				if mParam.Default == "" && len(parameter.Options) > 0 {
					// if the parameter has options (enum) and no default,
					// use the first element from the options.
					parameter.Default = parameter.Options[0]
				}

				newParameters[name] = Parameter{
					Alias:       mParam.Alias,
					Type:        mParam.Type,
					Description: mParam.Description,
					Placeholder: parameter.Placeholder,
					Required:    mParam.Required,
					Default:     mParam.Default,
					Pattern:     parameter.Pattern,
					Options:     parameter.Options,
					Index:       mParam.Index,
					Format:      parameter.Format,
					IsMulti:     mParam.IsMulti,
				}
			}
		}
	}

	newAction.Parameters = newParameters
	newAction.Alias = maskedAct.Alias

	return newAction
}

// IsPrefix checks if the given name is a prefix and should not be added to the mask file.
// for example:
//	service:
//    alias: "Service"
//  service.acknowledgement_timeout:
//     alias: "Service Acknowledgement Timeout"
// the service is a prefix and should not be added.
func IsPrefix(act sdkPlugin.Action, name string) bool {
	for paramName := range act.Parameters {
		if strings.HasPrefix(paramName, name) && paramName != name {
			return true
		}
	}
	return false
}

// GetMaskedActions gets the actions from the mask file, when filterParameters is set to false it will return all the original parameters.
func GetMaskedActions(maskFile string, actions []sdkPlugin.Action, blacklistParams []string, filterParameters bool) ([]EnhancedAction, error) {
	for _, action := range actions {
		for paramName := range action.Parameters {
			if IsPrefix(action, paramName) || (len(blacklistParams) > 0 && StringInSlice(paramName, blacklistParams)) {
				delete(action.Parameters, paramName)
			}
		}
	}

	// mask file was not given
	if maskFile == "" {
		var a []EnhancedAction

		for _, action := range actions {
			a = append(a, newCliAction(action))
		}

		return a, nil
	}

	m, err := mask.ParseMask(maskFile)
	if err != nil {
		return nil, err
	}

	var newActions []EnhancedAction

	for name, maskedAct := range m.Actions {
		originalName := m.ReplaceActionAlias(name) // get the operationID
		for _, act := range actions {
			if act.Name == originalName { // only take the actions that appear in the mask.
				newActions = append(newActions, FilterMaskedParameters(maskedAct, act, filterParameters))
			}
		}
	}

	return newActions, nil
}

func writeActions(actions []EnhancedAction, outputFileName string) error {
	sort.SliceStable(actions, func(i, j int) bool { // sort the actions before writing them for consistency.
		return actions[i].Name < actions[j].Name
	})

	f, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer f.Close()

	err = runTemplate(f, YAMLTemplate, actions)
	if err != nil {
		return err
	}

	return nil
}

func GenerateMaskFile(c *cli.Context) error {
	apiPlugin, err := plugin.NewOpenApiPlugin(nil, plugin.PluginMetadata{
		OpenApiFile: c.String("file"),
	}, plugin.Callbacks{})
	if err != nil {
		return err
	}

	actions, err := GetMaskedActions(c.String("mask"), apiPlugin.GetActions(), c.StringSlice("blacklist-params"), c.Bool("filterParameters"))
	if err != nil {
		return err
	}

	if !c.Bool("no-warnings") { // if warnings are enabled
		a := fmt.Sprintf("You are about to generate [%d] actions \nwith blacklist of %q\nand use mask original mask parameters set to [%#v]\n", len(actions), c.StringSlice("blacklist-params"), c.Bool("filterParameters"))

		fmt.Println(a)
		prompt := promptui.Prompt{
			Label:     "Are you sure",
			Default:   "Y",
			IsConfirm: true,
		}

		result, err := prompt.Run()
		if err != nil {
			return err
		}

		if result == "n" {
			return errors.New("")
		}
	}

	outputFileName := c.String("output")

	fmt.Printf("Generated [%d] actions into [%s]\n", len(actions), outputFileName)
	err = writeActions(actions, outputFileName)
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

	f, err := os.Create(README)
	if err != nil {
		return err
	}
	defer f.Close()

	err = runTemplate(f, READMETemplate, apiPlugin)
	if err != nil {
		return err
	}

	return nil
}

// GenerateAction appends a single ParameterName to mask file.
func GenerateAction(c *cli.Context) error {
	return _generateAction(c.String("name"), c.String("file"), c.String("output"), c.StringSlice("blacklist-params"), c.String("interactive"))
}

func _generateAction(actionName string, OpenApiFile string, outputFileName string, paramBlacklist []string, isInteractive string) error {
	apiPlugin, err := plugin.NewOpenApiPlugin(nil, plugin.PluginMetadata{
		OpenApiFile: OpenApiFile,
	}, plugin.Callbacks{})
	if err != nil {
		return err
	}

	maskedActions, err := GetMaskedActions(outputFileName, apiPlugin.GetActions(), paramBlacklist, true)
	if err != nil {
		return err
	}

	newActionPtr := FilterActionsByOperationName(actionName, apiPlugin.GetActions()) // get the specific ParameterName we want to generate.

	if newActionPtr == nil {
		return errors.New("no such ParameterName")
	}

	newAction := newCliAction(*newActionPtr)

	fmt.Printf("Adding %s...\n", actionName)

	if val, _ := strconv.ParseBool(isInteractive); val {
		InteractivelyFilterParameters(&newAction)
	}

	actions := replaceOldActionWithNew(maskedActions, newAction)

	err = writeActions(actions, outputFileName)
	if err != nil {
		return err
	}

	fmt.Printf("Generated [%s] into [%s]\n", actionName, outputFileName)
	return nil
}

func InteractivelyFilterParameters(action *EnhancedAction) {
	const (
		paramRequired = "Required"
		paramOptional = "Optional"
		discardParam  = "Discard"
	)

	newParameters := map[string]Parameter{}

	templates := promptui.SelectTemplates{
		Active:   `🍔 {{ . | green | bold }}`,
		Inactive: `   {{ . }}`,
		Label:    `Add {{ . | blue | bold}}:`,
	}

	for name, param := range action.Parameters {

		templates.Selected = `{{ "✔" | green | bold }} {{ "Parameter" | bold }} {{ "` + name + `" | bold }}: {{if eq . "` + paramRequired + `"}} {{ . | magenta }}  {{else if eq . "` + discardParam + `"}} {{ . | red }} {{else}} {{ . | cyan }} {{end}}`

		prompt := promptui.Select{
			Label:     name,
			Templates: &templates,
			Items:     []string{paramRequired, paramOptional, discardParam},
		}

		_, result, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		switch result {
		case paramRequired:
			param.Required = true
		case discardParam:
			continue
		}

		newParameters[name] = param
	}

	action.Parameters = newParameters
}

// replaceOldActionWithNew filters out the old ParameterName from the actions slice, and adds the newCliAction to the end.
func replaceOldActionWithNew(actions []EnhancedAction, newAction EnhancedAction) []EnhancedAction {
	var NewArray []EnhancedAction

	for _, act := range actions {
		// go over the actions and take all the actions that dont match with the new ParameterName.
		if act.Name != newAction.Name {
			NewArray = append(NewArray, act)
		}
	}

	NewArray = append(NewArray, newAction)

	return NewArray
}

func FilterActionsByOperationName(operationName string, actions []sdkPlugin.Action) *sdkPlugin.Action {
	for _, action := range actions {
		if action.Name == operationName {
			return &action
		}
	}

	return nil
}

func genAlias(str string) string {
	upperCaseWords := []string{"url", "id", "ids", "ip", "ssl"}

	// replace _ with ' '
	str = strings.ReplaceAll(str, "_", " ")
	str = strings.ReplaceAll(str, ".", " ")
	str = strings.ReplaceAll(str, "[]", "")
	// iter over words in the string

	words := strings.Split(str, " ")

	for i, word := range words {
		// check if the word is in out list.
		if plugin.StringInSlice(word, upperCaseWords) {
			words[i] = strings.ToUpper(word)
		}
	}

	str = strings.Join(words, " ")
	str = strings.ReplaceAll(str, "IDS", "IDs")

	return strings.Join(strings.Fields(strings.Title(str)), " ")
}

func runTemplate(f io.Writer, templateStr string, obj interface{}) error {
	indexMap := map[string]int{}

	funcs := template.FuncMap{
		"genAlias": genAlias,
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
		"fixType": func(str string) string {
			return strings.ReplaceAll(str, "-", "_")
		},
		"notEmpty": func(str string) bool {
			return len(str) > 0
		},
	}

	tmpl, err := template.New("").Funcs(funcs).Parse(templateStr)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(f, obj); err != nil {
		return err
	}

	return nil
}
