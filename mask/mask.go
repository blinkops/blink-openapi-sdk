package mask

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

var (
	reverseActionAliasMap    = map[string]string{}
	reverseParameterAliasMap = map[string]map[string]string{}
	MaskData                 = &Mask{}
)

type Mask struct {
	Actions map[string]*MaskedAction `yaml:"actions,omitempty"`
}

type MaskedAction struct {
	Alias      string                           `yaml:"alias,omitempty"`
	Parameters map[string]*MaskedActionParameter `yaml:"parameters,omitempty"`
}

type MaskedActionParameter struct {
	Alias string `yaml:"alias,omitempty"`
}

func ParseMask(maskFile string) error {
	if maskFile == "" {
		return nil
	}

	maskData, err := ioutil.ReadFile(maskFile)

	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(maskData, MaskData); err != nil {
		return err
	}

	buildActionAliasMap()
	buildParamAliasMap()

	return nil
}

func buildActionAliasMap() {
	for originalName, actionData := range MaskData.Actions {
		if actionData.Alias != "" {
			reverseActionAliasMap[actionData.Alias] = originalName
		}
	}
}

func buildParamAliasMap() {
	for actionName, actionData := range MaskData.Actions {
		reverseParameterAliasMap[actionName] = map[string]string{}

		for originalName, parameterData := range actionData.Parameters {
			if parameterData.Alias != "" {
				reverseParameterAliasMap[actionName][parameterData.Alias] = originalName
			}
		}
	}
}

func ReplaceActionAlias(actionName string) string {
	if originalName, ok := reverseActionAliasMap[actionName]; ok {
		return originalName
	}

	return actionName
}

func ReplaceActionParameterAlias(actionName string, paramName string) string {
	if actionParams, ok := reverseParameterAliasMap[actionName]; ok {
		if originalName, ok := actionParams[paramName]; ok {
			return originalName
		}
	}

	return paramName
}

func ReplaceActionParametersAliases(originalActionName string, rawParameters map[string]string) map[string]string {
	requestParameters := map[string]string{}

	for paramName, paramValue := range rawParameters {
		originalName := ReplaceActionParameterAlias(originalActionName, paramName)
		requestParameters[originalName] = paramValue
	}

	return requestParameters
}
