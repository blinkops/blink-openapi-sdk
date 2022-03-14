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
	"unicode"

	"github.com/AlecAivazis/survey/v2"

	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-openapi-sdk/plugin"
	sdkPlugin "github.com/blinkops/blink-sdk/plugin"
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

	newParameters := []GeneratedParameter{}

	for parmName, mParam := range maskedAct.Parameters {
		for name, parameter := range act.Parameters {
			if name == parmName { // if the action name is also in the mask file.
				if mParam.Default == "" && len(parameter.Options) > 0 {
					// if the parameter has options (enum) and no default,
					// use the first element from the options.
					parameter.Default = parameter.Options[0]
				}

				if mParam.Description == "" {
					mParam.Description = parameter.Description
				}

				newParameters = append(newParameters, GeneratedParameter{
					Name:        name,
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
				})
			}
		}
	}

	sort.SliceStable(newParameters, func(i, j int) bool {
		return newParameters[i].Index < newParameters[j].Index
	})

	newAction.Parameters = newParameters
	if maskedAct.Alias != "" {
		newAction.Alias = maskedAct.Alias
	} else {
		newAction.Alias = genAlias(newAction.Name)
	}

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

		prompt := &survey.Confirm{
			Message: "Are you sure?",
		}

		result := false

		err = survey.AskOne(prompt, &result)
		if err != nil {
			return err
		}

		if !result {
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
		OpenApiFile: openapiFile,
		Name:        pluginName,
	}, plugin.Callbacks{})
	if err != nil {
		return err
	}

	actions, err := GetMaskedActions(maskFile, apiPlugin.GetActions(), []string{}, true)
	if err != nil {
		return err
	}
	sort.SliceStable(actions, func(i, j int) bool { // sort the actions before writing them for consistency.
		return actions[i].Name < actions[j].Name
	})

	pluginMask := GeneratedReadme{
		Name:        apiPlugin.Describe().Name,
		Description: apiPlugin.Describe().Description,
		Actions:     actions,
	}

	f, err := os.Create(README)
	if err != nil {
		return err
	}

	defer f.Close()

	if customActionsPath != "" {
		pluginMask.Actions = append(pluginMask.Actions, generateCustomActionsReadme(customActionsPath)...)
	}

	err = runTemplate(f, READMETemplate, pluginMask)
	if err != nil {
		return err
	}

	return nil
}

func generateCustomActionsReadme(path string) []GeneratedAction {
	b := []GeneratedAction{}

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

		b = append(b, newGeneratedAction(action))

		return nil
	})
	if err != nil {
		log.Error("Error occurred while going over the custom actions: " + err.Error())
	}
	return b
}

// GenerateAction appends a single ParameterName to mask file.
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

func _fixMask(path string) error {
	mask, err := mask.ParseMask(path)
	if err != nil {
		return err
	}

	for actionName, action := range mask.Actions {
		if action.DisplayName == "" {
			cleanActionName := getDisplayName(actionName)
			fmt.Println(cleanActionName)
		}
		for paramName, param := range action.Parameters {
			if strings.Contains(paramName, ".") {
				delete(action.Parameters, paramName)
				newParamName := strings.ReplaceAll(paramName, ".", "__")
				action.Parameters[newParamName] = param
			}
		}
	}

	maskContent, err := yaml.Marshal(mask)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, maskContent, 0644)
	if err != nil {
		return err
	}

	return nil
}

func contains(list []string, word string) bool {
	for _, item := range list {
		if item == word {
			return true
		}
	}
	return false
}

func remove(slice []string, i int) []string {
	return append(slice[:i], slice[i+1:]...)
}

func removeDuplicates(words []string) []string {
	if len(words) > 3 {
		if strings.ToLower(words[0] + words[1]) == strings.ToLower(words[2]) {
			words = remove(words, 2)
		}
	}

	if len(words) > 4 {
		var indexesToDelete []int
		for i := 0; i+3 < len(words); i++ {
			if strings.ToLower(words[i]+words[i+1]) == strings.ToLower(words[i+2]+words[i+3]) {
				indexesToDelete = append(indexesToDelete, []int{i, i+1}...)
			}
		}

		for i, indexToDelete := range indexesToDelete {
			indexToDelete = indexToDelete - i
			words = remove(words, indexToDelete)
		}
	}

	return words
}

func getDisplayName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, ":", " ")
	name = strings.ReplaceAll(name, "[]", "")
	for i := 1; i < len(name); i++ {
		if unicode.IsLower(rune(name[i-1])) && unicode.IsUpper(rune(name[i])) && (i+1 < len(name) || !unicode.IsUpper(rune(name[i+1]))) {
			name = name[:i] + " " + name[i:]
		}
	}
	upperCaseWords := []string{"url", "id", "ids", "ip", "ssl"}
	words := strings.Split(name, " ")
	for i, word := range words {
		if contains(upperCaseWords, word) {
			words[i] = strings.ToUpper(word)
		}
	}

	words = removeDuplicates(words)

	name = strings.Join(words, " ")
	name = strings.ReplaceAll(name, "IDS", "IDs")
	return strings.Join(strings.Fields(strings.Title(name)), " ")
}

func InteractivelyFilterParameters(action *GeneratedAction) {
	newParameters := []GeneratedParameter{}

	paramNames := []string{}
	for _, parameter := range action.Parameters {
		paramNames = append(paramNames, parameter.Name)
	}

	selectedParams := []string{}
	prompt := &survey.MultiSelect{
		Message: "Select Parameters",
		Options: paramNames,
	}
	err := survey.AskOne(prompt, &selectedParams)
	if err != nil {
		log.Fatal(err)
	}

	requiredParams := []string{}
	prompt = &survey.MultiSelect{
		Message: "Select required Parameters",
		Options: selectedParams,
	}
	err = survey.AskOne(prompt, &requiredParams)
	if err != nil {
		log.Fatal(err)
	}

	for _, parameter := range action.Parameters {
		if StringInSlice(parameter.Name, selectedParams) {

			if StringInSlice(parameter.Name, requiredParams) {
				parameter.Required = true
			}
			newParameters = append(newParameters, parameter)
		}
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
