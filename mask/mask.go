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

func (m *Mask) GetAction(actionName string) *MaskedAction {
	if action, ok := m.Actions[ReplaceActionAlias(actionName)]; ok {
		return action
	}

	return nil
}

func (m *Mask) GetParameter(actionName string, paramName string) *MaskedActionParameter {
	originalActionName := ReplaceActionAlias(actionName)

	if action, ok := m.Actions[ReplaceActionAlias(originalActionName)]; ok {
		if param, ok := action.Parameters[replaceActionParameterAlias(originalActionName, paramName)]; ok {
			return param
		}

		return nil
	}

	return nil
}

func (m *Mask) IsParamRequired(actionName string, paramName string) string {
	paramMask := MaskData.GetParameter(actionName, paramName)

	if paramMask != nil && paramMask.Required != "" {
		return paramMask.Required
	}

	return ""
}

type MaskedAction struct {
	Alias      string                            `yaml:"alias,omitempty"`
	Parameters map[string]*MaskedActionParameter `yaml:"parameters,omitempty"`
}

type MaskedActionParameter struct {
	Alias    string `yaml:"alias,omitempty"`
	Required string `yaml:"required,omitempty"`
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

func replaceActionParameterAlias(actionName string, paramName string) string {
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
		originalName := replaceActionParameterAlias(originalActionName, paramName)
		requestParameters[originalName] = paramValue
	}

	return requestParameters
}
