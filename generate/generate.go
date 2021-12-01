package gen

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Songmu/prompter"
	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-openapi-sdk/plugin"
	sdkPlugin "github.com/blinkops/blink-sdk/plugin"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	Action = `{{range $ParameterName := .}}
  {{$ParameterName.Name }}:
    alias: {{ actName $ParameterName.Name }}
    parameters:{{ range $name, $param := .Parameters}}
      {{ if badPrefix $name }}"{{$name}}":
      {{- else }}{{$name}}:{{end}}
        alias: "{{ paramName $name }}"
        {{- if $param.Required }}
        required: true{{end}}
		{{- if notEmpty $param.Default }}
        default: {{$param.Default}}{{end}}
		{{- if $param.Format}} 
        type: {{ fixType $param.Format }}{{end}}
        index: {{ index $ParameterName.Name }}{{ end}}{{ end}}`

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

// FilterMaskedParameters returns a new ParameterName with the same parameters as the masked ParameterName.
func FilterMaskedParameters(maskedAct *mask.MaskedAction, act sdkPlugin.Action, filterParameters bool) sdkPlugin.Action {
	if !filterParameters { // return the original action

		return act
	}

	newParameters := map[string]sdkPlugin.ActionParameter{}

	for parmName, mParam := range maskedAct.Parameters {
		for name, parameter := range act.Parameters {
			if name == parmName { // if the ParameterName name is also in the mask file.

				// if the mask stated that this param should be required.
				if mParam.Required {
					parameter.Required = mParam.Required
				}
				if mParam.Default != "" { // take the default value for the action from the mask.
					parameter.Default = mParam.Default
				} else if len(parameter.Options) > 0 {
					// if the parameter has options (enum) and no default,
					// use the first element from the options.
					parameter.Default = parameter.Options[0]
				}

				newParameters[name] = parameter
			}
		}
	}
	act.Parameters = newParameters
	return act
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
func GetMaskedActions(maskFile string, actions []sdkPlugin.Action, blacklistParams []string, filterParameters bool) ([]sdkPlugin.Action, error) {
	for _, action := range actions {
		for paramName := range action.Parameters {
			if IsPrefix(action, paramName) || (len(blacklistParams) > 0 && StringInSlice(paramName, blacklistParams)) {
				delete(action.Parameters, paramName)
			}
		}
	}

	// mask file was not given
	if maskFile == "" {
		return actions, nil
	}

	m, err := mask.ParseMask(maskFile)
	if err != nil {
		return nil, err
	}

	var newActions []sdkPlugin.Action

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

func writeActions(actions []sdkPlugin.Action, outputFileName string) error {
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
	apiPlugin, err := plugin.NewOpenApiPlugin(nil, plugin.PluginMetadata{
		OpenApiFile: c.String("file"),
	}, plugin.Callbacks{})
	if err != nil {
		return err
	}

	outputFileName := c.String("output")

	maskedActions, err := GetMaskedActions(outputFileName, apiPlugin.GetActions(), c.StringSlice("blacklist-params"), true)
	if err != nil {
		return err
	}

	actionName := c.String("name")
	newAction := FilterActionsByOperationName(actionName, apiPlugin.GetActions()) // get the specific ParameterName we want to generate.

	if newAction == nil {
		return errors.New("no such ParameterName")
	}

	fmt.Printf("Adding %s...\n", actionName)

	if val, _ := strconv.ParseBool(c.String("interactive")); val {
		InteractivelyFilterParameters(newAction)
	}

	actions := replaceOldActionWithNew(maskedActions, *newAction)

	err = writeActions(actions, outputFileName)

	if err != nil {
		return err
	}

	fmt.Printf("Generated [%s] into [%s]\n", actionName, outputFileName)
	return nil
}

func InteractivelyFilterParameters(action *sdkPlugin.Action) {
	const (
		paramRequired = "Required"
		paramOptional = "Optional"
		discardParam  = "Discard"
	)

	newParameters := map[string]sdkPlugin.ActionParameter{}

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

// replaceOldActionWithNew filters out the old ParameterName from the actions slice, and adds the newAction to the end.
func replaceOldActionWithNew(actions []sdkPlugin.Action, newAction sdkPlugin.Action) []sdkPlugin.Action {
	var NewArray []sdkPlugin.Action

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

func runTemplate(f io.Writer, templateStr string, obj interface{}) error {
	upperCaseWords := []string{"url", "id", "ids", "ip", "ssl"}
	indexMap := map[string]int{}

	genAlias := func(str string) string {
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

	funcs := template.FuncMap{
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
