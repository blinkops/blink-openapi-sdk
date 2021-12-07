package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/getkin/kin-openapi/openapi3"
	log "github.com/sirupsen/logrus"
)

func handleBodyParams(metadata bodyMetadata, paramSchema *openapi3.Schema, parentPath string, schemaPath string, parentsRequired bool) {
	handleBodyParamOfType(metadata, paramSchema, parentPath, schemaPath, parentsRequired)

	for propertyName, bodyProperty := range paramSchema.Properties {
		fullParamPath, fullSchemaPath := propertyName, schemaPath

		if bodyProperty.Ref != "" {
			index := strings.LastIndex(bodyProperty.Ref, "/") + 1
			fullSchemaPath += bodyProperty.Ref[index:] + "."
			if hasDuplicateSchemas(fullSchemaPath) {
				continue
			}
		}

		// Json params are represented as dot delimited params to allow proper parsing in UI later on
		if parentPath != "" {
			fullParamPath = parentPath + consts.BodyParamDelimiter + fullParamPath
		}

		// Keep recursion until leaf node is found
		if bodyProperty.Value.Properties != nil {
			handleBodyParams(metadata, bodyProperty.Value, fullParamPath, fullSchemaPath, areParentsRequired(parentsRequired, propertyName, paramSchema))
		} else {
			handleBodyParamOfType(metadata, bodyProperty.Value, fullParamPath, fullSchemaPath, parentsRequired)
			isParamRequired := false

			for _, requiredParam := range paramSchema.Required {
				if propertyName == requiredParam {
					isParamRequired = parentsRequired
					break
				}
			}

			if actionParam := parseActionParam(metadata.maskData, metadata.action.Name, &fullParamPath, bodyProperty, isParamRequired, bodyProperty.Value.Description); actionParam != nil {
				metadata.action.Parameters[fullParamPath] = *actionParam
			}
		}
	}
}

func handleBodyParamOfType(metadata bodyMetadata, paramSchema *openapi3.Schema, parentPath string, schemaPath string, parentsRequired bool) {
	if paramSchema.AllOf != nil || paramSchema.AnyOf != nil || paramSchema.OneOf != nil {

		allSchemas := []openapi3.SchemaRefs{paramSchema.AllOf, paramSchema.AnyOf, paramSchema.OneOf}

		// find properties nested in Allof, Anyof, Oneof
		for _, schemaType := range allSchemas {
			for _, schemaParams := range schemaType {
				handleBodyParams(metadata, schemaParams.Value, parentPath, schemaPath, parentsRequired)
			}
		}
	}
}

func parseActionParam(maskData mask.Mask, actionName string, paramName *string, paramSchema *openapi3.SchemaRef, isParamRequired bool, paramDescription string) *plugin.ActionParameter {
	var (
		isMulti    bool
		paramIndex int64
	)

	paramType := paramSchema.Value.Type
	paramFormat := paramSchema.Value.Format

	paramOptions := getParamOptions(paramSchema.Value.Enum, &paramType)
	paramPlaceholder := getParamPlaceholder(paramSchema.Value.Example, paramType)
	paramDefault := getParamDefault(paramSchema.Value.Default, paramType)
	paramIndex = 999 // parameters will be ordered from lowest to highest in UI. This is the default, meaning - the end of the list.

	if maskData.Actions != nil {
		maskedParam := maskData.GetParameter(actionName, *paramName)
		if maskedParam == nil {
			return nil
		}
		if maskedParam.Alias != "" {
			*paramName = maskedParam.Alias
		}

		// Override Required property only if not explicitly defined by OpenAPI definition
		if !isParamRequired {
			isParamRequired = maskedParam.Required
		}

		// Override the Type property
		if maskedParam.Type != "" {
			extractedType := extractTypeFromFormat(maskedParam.Type)

			if extractedType == "" {
				paramType = maskedParam.Type
			} else {
				paramType = extractedType
				paramFormat = maskedParam.Type
			}
		}

		if maskedParam.Index != 0 {
			paramIndex = maskedParam.Index
		}

		if maskedParam.IsMulti {
			isMulti = true
		}

		if maskedParam.Default != "" {
			paramDefault = maskedParam.Default

			if paramType == consts.TypeJson {
				defaultMarshal := new(bytes.Buffer)
				if err := json.Indent(defaultMarshal, []byte(paramDefault), "", "\t"); err == nil {
					paramDefault = defaultMarshal.String()
				} else {
					log.Debugf("Failed to marshal default value: %s, got: %v", paramDefault, err)
				}
			}
		}
	}

	// Convert parameters of type object to code:json and parameters of type boolean to bool
	convertParamType(&paramType)

	return &plugin.ActionParameter{
		Type:        paramType,
		Description: paramDescription,
		Placeholder: paramPlaceholder,
		Required:    isParamRequired,
		Default:     paramDefault,
		Options:     paramOptions,
		Index:       paramIndex,
		Format:      paramFormat,
		IsMulti:     isMulti,
	}
}

func areParentsRequired(parentsRequired bool, propertyName string, schema *openapi3.Schema) bool {
	if !parentsRequired {
		return false
	}

	for _, requiredParam := range schema.Required {
		if propertyName == requiredParam {
			return true
		}
	}

	return false
}

func getParamOptions(parsedOptions []interface{}, paramType *string) []string {
	paramOptions := []string{}

	if parsedOptions == nil {
		return nil
	}

	for _, option := range parsedOptions {
		if optionString, ok := option.(string); ok {
			paramOptions = append(paramOptions, optionString)
		}
	}

	if len(paramOptions) > 0 {
		*paramType = consts.TypeDropdown
	}

	return paramOptions
}

func getParamPlaceholder(paramExample interface{}, paramType string) string {
	paramPlaceholder, _ := paramExample.(string)

	if paramType != consts.TypeObject {
		if paramPlaceholder != "" {
			return consts.ParamPlaceholderPrefix + paramPlaceholder
		}
	}

	return paramPlaceholder
}

func getParamDefault(defaultValue interface{}, paramType string) string {
	var paramDefault string

	if paramType != consts.TypeArray {
		if defaultValue == nil {
			paramDefault = ""
		} else {
			paramDefault = fmt.Sprintf("%v", defaultValue)
		}

		return paramDefault
	}

	if defaultList, ok := defaultValue.([]interface{}); ok {
		var defaultStrings []string

		for _, value := range defaultList {
			valueString := fmt.Sprintf("%v", value)
			defaultStrings = append(defaultStrings, valueString)
		}

		paramDefault = strings.Join(defaultStrings, consts.ArrayDelimiter)
	}

	return paramDefault
}

func hasDuplicateSchemas(path string) bool {
	paramsArray := strings.Split(path, consts.BodyParamDelimiter)
	exists := make(map[string]bool)
	for _, param := range paramsArray {
		if exists[param] {
			return true
		} else {
			exists[param] = true
		}
	}
	return false
}

func convertParamType(paramType *string) {
	switch *paramType {
	case consts.TypeObject:
		*paramType = consts.TypeJson
	case consts.TypeBoolean:
		*paramType = consts.TypeBool
	}
}

func extractTypeFromFormat(paramFormat string) string {
	paramType := strings.Split(paramFormat, mask.FormatDelimiter)[0]

	for _, prefixType := range mask.FormatPrefixes {
		if paramType == prefixType {
			return paramType
		}
	}

	return ""
}
