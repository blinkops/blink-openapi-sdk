package gen

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Songmu/prompter"
	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-openapi-sdk/plugin"
	sdkPlugin "github.com/blinkops/blink-sdk/plugin"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	Action = `{{range $action := .}}
  {{$action.Name }}:
    alias: {{ actName $action.Name }}
    parameters:{{ range $name, $param := .Parameters}}
      {{ if badPrefix $name }}"{{$name}}":
      {{- else }}{{$name}}:{{end}}
        alias: "{{ paramName $name }}"
        {{- if $param.Required }}
        required: true{{end}}
        {{- if $param.Format }}
        type: {{ fixType $param.Format }}{{end}}
        index: {{ index $action.Name }}{{ end}}{{ end}}`

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

// FilterMaskedParameters returns a new action with the same parameters as the masked action.
func FilterMaskedParameters(maskedAct *mask.MaskedAction, act sdkPlugin.Action, filterParameters bool) sdkPlugin.Action {
	if !filterParameters {
		return act
	}

	newParameters := map[string]sdkPlugin.ActionParameter{}

	for parmName := range maskedAct.Parameters {
		for name, parameter := range act.Parameters {
			if name == parmName { // if the action name is also in the mask file.
				newParameters[name] = parameter
			}
		}
	}
	act.Parameters = newParameters
	return act
}

func GetMaskedActions(maskFile string, actions []sdkPlugin.Action, blacklistParams []string, filterParameters bool) ([]sdkPlugin.Action, error) {
	if len(blacklistParams) > 0 {
		for _, action := range actions {
			for paramName := range action.Parameters {
				if StringInSlice(paramName, blacklistParams) {
					delete(action.Parameters, paramName)
				}
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
		originalName := m.ReplaceActionAlias(name)
		for _, act := range actions {
			if act.Name == originalName {
				newActions = append(newActions, FilterMaskedParameters(maskedAct, act, filterParameters))
			}
		}
	}

	return newActions, nil
}

func writeActionsToTemplate(actions []sdkPlugin.Action, outputFileName string) error {
	sort.SliceStable(actions, func(i, j int) bool {
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

	if !c.Bool("no-warnings") { // user wants warnings
		a := fmt.Sprintf("You are about to generate [%d] actions \n with blacklist of [%#v] use mask original mask parameters [%#v] \n with are you sure?\n", len(actions), c.StringSlice("blacklist-params"), c.Bool("filterParameters"))

		if !prompter.YN(a, true) { // warn the user
			return errors.New("user quit")
		}
	}

	outputFileName := c.String("output")

	fmt.Printf("Generated [%d] actions into [%s]\n", len(actions), outputFileName)

	err = writeActionsToTemplate(actions, outputFileName)
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

// GenerateAction appends a single action to mask file.
func GenerateAction(c *cli.Context) error {
	apiPlugin, err := plugin.NewOpenApiPlugin(nil, plugin.PluginMetadata{
		OpenApiFile: c.String("file"),
	}, plugin.Callbacks{})
	if err != nil {
		return err
	}

	maskedActions, err := GetMaskedActions(c.String("output"), apiPlugin.GetActions(), c.StringSlice("blacklist-params"), true)
	if err != nil {
		return err
	}
	actionName := c.String("name")
	newAction := FilterActionsByOperationName(actionName, apiPlugin.GetActions())

	if len(newAction) == 0 {
		return errors.New("no such action")
	}

	fmt.Printf("Adding %s...\n", actionName)

	outputFileName := c.String("output")

	actions := replaceOldActionWithNew(maskedActions, newAction)

	err = writeActionsToTemplate(actions, outputFileName)
	if err != nil {
		return err
	}

	fmt.Printf("Generated [%s] into [%s]\n", actionName, outputFileName)
	return nil
}

// replaceOldActionWithNew filters out the old action from the actions slice, and adds the newAction to the end.
func replaceOldActionWithNew(actions []sdkPlugin.Action, newAction []sdkPlugin.Action) []sdkPlugin.Action {
	var NewArray []sdkPlugin.Action

	for _, act := range actions {
		if act.Name != newAction[0].Name {
			NewArray = append(NewArray, act)
		}
	}

	NewArray = append(NewArray, newAction[0])

	return NewArray
}

func FilterActionsByOperationName(operationName string, actions []sdkPlugin.Action) []sdkPlugin.Action {
	var filteredActions []sdkPlugin.Action

	for _, action := range actions {
		if action.Name == operationName {
			filteredActions = append(filteredActions, action)
		}
	}

	return filteredActions
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
