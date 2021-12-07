package gen

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-openapi-sdk/plugin"
	sdkPlugin "github.com/blinkops/blink-sdk/plugin"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
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
func FilterMaskedParameters(maskedAct *mask.MaskedAction, act sdkPlugin.Action, filterParameters bool) GeneratedAction {
	newAction := newGeneratedAction(act)

	if !filterParameters { // return the original action
		return newAction
	}

	newParameters := map[string]GeneratedParameter{}

	for parmName, mParam := range maskedAct.Parameters {
		for name, parameter := range act.Parameters {
			if name == parmName { // if the action name is also in the mask file.
				if mParam.Default == "" && len(parameter.Options) > 0 {
					// if the parameter has options (enum) and no default,
					// use the first element from the options.
					parameter.Default = parameter.Options[0]
				}

				newParameters[name] = GeneratedParameter{
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
func GetMaskedActions(maskFile string, actions []sdkPlugin.Action, blacklistParams []string, filterParameters bool) ([]GeneratedAction, error) {
	for _, action := range actions {
		for paramName := range action.Parameters {
			if IsPrefix(action, paramName) || (len(blacklistParams) > 0 && StringInSlice(paramName, blacklistParams)) {
				delete(action.Parameters, paramName)
			}
		}
	}

	// mask file was not given
	if maskFile == "" {
		var a []GeneratedAction

		for _, action := range actions {
			a = append(a, newGeneratedAction(action))
		}

		return a, nil
	}

	m, err := mask.ParseMask(maskFile)
	if err != nil {
		return nil, err
	}

	var newActions []GeneratedAction

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

func generateCustomActionsReadme(file io.Writer, path string) {
	currentDirectory, err := os.Getwd()
	if err != nil {
		log.Error(err.Error())
		return
	}
	err = filepath.WalkDir(currentDirectory+path, func(filePath string, _ fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(filePath, actionSuffix) {
			return nil
		}
		actionFile, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Error("Failed to read custom action file: " + err.Error())
			return err
		}
		var action sdkPlugin.Action
		err = yaml.Unmarshal(actionFile, &action)
		if err != nil {
			log.Error("Failed to unmarshal custom action: " + err.Error())
			return err
		}
		err = runTemplate(file, READMEAction, action)
		if err != nil {
			log.Error(err.Error())
			return err
		}
		return nil
	})
	if err != nil {
		log.Error("Error occurred while going over the custom actions: " + err.Error())
	}
}

func _generateMaskFile(OpenApiFile string, maskFile string, paramBlacklist []string, outputFileName string, filter bool, warnings bool) error {
	apiPlugin, err := plugin.NewOpenApiPlugin(nil, plugin.PluginMetadata{
		OpenApiFile: OpenApiFile,
	}, plugin.Callbacks{})
	if err != nil {
		return err
	}

	actions, err := GetMaskedActions(maskFile, apiPlugin.GetActions(), paramBlacklist, filter)
	if err != nil {
		return err
	}

	if !warnings { // if warnings are enabled
		a := fmt.Sprintf("You are about to generate [%d] actions \nwith blacklist of %q\nand use mask original mask parameters set to [%#v]\n", len(actions), paramBlacklist, filter)

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

	fmt.Printf("Generated [%d] actions into [%s]\n", len(actions), outputFileName)
	err = writeActions(actions, outputFileName)
	if err != nil {
		return err
	}

	return nil
}

func _GenerateReadme(pluginName string, maskFile string, openapiFile string, customActionsPath string) error {
	apiPlugin, err := plugin.NewOpenApiPlugin(nil, plugin.PluginMetadata{
		Name:        pluginName,
		MaskFile:    maskFile,
		OpenApiFile: openapiFile,
	}, plugin.Callbacks{})
	if err != nil {
		return err
	}

	f, err := os.OpenFile(README, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	defer f.Close()

	err = runTemplate(f, READMETemplate, apiPlugin)
	if err != nil {
		return err
	}

	if customActionsPath != "" {
		generateCustomActionsReadme(f, customActionsPath)
	}

	return nil
}


func generateCustomActionsReadme(file io.Writer, path string) {
	err := filepath.WalkDir(path, func(filePath string, _ fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(filePath, actionSuffix) {
			return nil
		}
		actionFile, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Error("Failed to read custom action file: " + err.Error())
			return err
		}
		var action sdkPlugin.Action
		err = yaml.Unmarshal(actionFile, &action)
		if err != nil {
			log.Error("Failed to unmarshal custom action: " + err.Error())
			return err
		}
		err = runTemplate(file, READMEAction, action)
		if err != nil {
			log.Error(err.Error())
			return err
		}
		return nil
	})
	if err != nil {
		log.Error("Error occurred while going over the custom actions: " + err.Error())
	}
}

// GenerateAction appends a single ParameterName to mask file.
func GenerateAction(c *cli.Context) error {
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

	newActionPtr := FilterActionsByOperationName(actionName, apiPlugin.GetActions()) // get the specific action we want to generate.

	if newActionPtr == nil {
		return errors.New("no such action")
	}

	newAction := newGeneratedAction(*newActionPtr)

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

func InteractivelyFilterParameters(action *GeneratedAction) {
	const (
		paramRequired = "Required"
		paramOptional = "Optional"
		discardParam  = "Discard"
	)

	newParameters := map[string]GeneratedParameter{}

	templates := promptui.SelectTemplates{
		Active:   `🍔 {{ . | green | bold }}`,
		Inactive: `   {{ . }}`,
		Label:    `Add {{ . | blue | bold}}:`,
	}

	for name, param := range action.Parameters {

		templates.Selected = `{{ "✔" | green | bold }} {{ "GeneratedParameter" | bold }} {{ "` + name + `" | bold }}: {{if eq . "` + paramRequired + `"}} {{ . | magenta }}  {{else if eq . "` + discardParam + `"}} {{ . | red }} {{else}} {{ . | cyan }} {{end}}`

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

// replaceOldActionWithNew filters out the old action from the actions slice, and adds the newGeneratedAction to the end.
func replaceOldActionWithNew(actions []GeneratedAction, newAction GeneratedAction) []GeneratedAction {
	var NewArray []GeneratedAction

	for _, act := range actions {
		// go over the actions and take all the actions that dont match with the new action.
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

func writeActions(actions []GeneratedAction, outputFileName string) error {
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

func genAlias(str string) string {
	upperCaseWords := []string{"url", "id", "ids", "ip", "ssl"}
	str = strings.ReplaceAll(str, "_", " ")
	str = strings.ReplaceAll(str, ".", " ")
	str = strings.ReplaceAll(str, "[]", "")

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
