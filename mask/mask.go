package mask

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

const (
	FormatDelimiter = "_"
)

var (
	reverseActionAliasMap    = map[string]string{}
	reverseParameterAliasMap = map[string]map[string]string{}
	MaskData                 = &Mask{}
	FormatPrefixes           = []string{"date"}
)

type (
	Mask struct {
		Actions map[string]*MaskedAction `yaml:"actions,omitempty"`
	}
	MaskedAction struct {
		Alias      string                            `yaml:"alias,omitempty"`
		Parameters map[string]*MaskedActionParameter `yaml:"parameters,omitempty"`
	}
	MaskedActionParameter struct {
		Alias    string `yaml:"alias,omitempty"`
		Required bool   `yaml:"required,omitempty"`
		Type     string `yaml:"type,omitempty"` // password/date - 2017-07-21/date_time - 2017-07-21T17:32:28Z/date_epoch - 1631399887
		Index    int    `yaml:"index,omitempty"`
		IsMulti  bool   `yaml:"is_multi,omitempty"` // is this a multi-select field
	}
)

func (m *Mask) GetAction(actionName string) *MaskedAction {
	originalActionName := ReplaceActionAlias(actionName)

	if action, ok := m.Actions[originalActionName]; ok {
		return action
	}

	return nil
}

func (m *Mask) GetParameter(actionName string, paramName string) *MaskedActionParameter {
	originalActionName := ReplaceActionAlias(actionName)
	originalParamName := replaceActionParameterAlias(actionName, paramName)

	if action, ok := m.Actions[originalActionName]; ok {
		if param, ok := action.Parameters[originalParamName]; ok {
			return param
		}

		return nil
	}

	return nil
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

			if _, ok := reverseActionAliasMap[actionData.Alias]; ok {
				// error
				log.Fatalln("Alias " + actionData.Alias + " exist multiple times.")
			}

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
