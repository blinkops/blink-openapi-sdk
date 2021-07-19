package mask

import (
	"fmt"
	"github.com/blinkops/blink-openapi-sdk/plugin"
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	"github.com/go-yaml/yaml"
	"github.com/pkg/errors"
	"io/ioutil"
)

var (
	reverseActionAliasMap    = map[string]string{}
	reverseParameterAliasMap = map[string]map[string]string{}
)

type Mask struct {
	Actions map[string]*MaskedAction `yaml:"actions"`
}

type MaskedAction struct {
	Alias      string                            `yaml:"alias"`
	Parameters map[string]*MaskedActionParameter `yaml:"parameters"`
}

type MaskedActionParameter struct {
	Alias string `yaml:"alias"`
}

func ParseMask(maskFile string) error {
	if maskFile == "" {
		return nil
	}

	maskData, err := ioutil.ReadFile(maskFile)

	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(maskData, plugin.MaskData); err != nil {
		return err
	}

	buildActionAliasMap()
	buildParamAliasMap()

	return nil
}

func buildActionAliasMap() {
	for originalName, actionData := range plugin.MaskData.Actions {
		if actionData.Alias != "" {
			reverseActionAliasMap[actionData.Alias] = originalName
		}
	}
}

func buildParamAliasMap() {
	for actionName, actionData := range plugin.MaskData.Actions {
		reverseParameterAliasMap[actionName] = map[string]string{}

		for originalName, parameterData := range actionData.Parameters {
			if parameterData.Alias != "" {
				reverseParameterAliasMap[actionName][parameterData.Alias] = originalName
			}
		}
	}
}

func ReplaceActionAlias(actionName string) (string, error) {
	// Check if no alias was used for this action
	if _, ok := plugin.OperationDefinitions[actionName]; ok {
		return actionName, nil
	}

	// Check the alias for this action
	if originalName, ok := reverseActionAliasMap[actionName]; ok {
		return originalName, nil
	}

	return "", errors.New(fmt.Sprintf("Couldn't find action for alias: %s", actionName))
}

func ReplaceActionParametersAliases(originalActionName string, rawParameters map[string]string, operation *handlers.OperationDefinition) map[string]string {
	requestParameters := map[string]string{}

	for paramName, paramValue := range rawParameters {
		// Check if no alias was used for this parameter
		for _, pathParam := range operation.AllParams() {
			if pathParam.ParamName == paramName {
				requestParameters[paramName] = paramValue
				break
			}
		}

		// Parameter was found by its original name
		if _, ok := requestParameters[paramName]; ok {
			continue
		}

		// Check the alias for this parameter
		if originalName, ok := reverseParameterAliasMap[originalActionName][paramName]; ok {
			requestParameters[originalName] = paramValue
		}
	}

	return requestParameters
}
