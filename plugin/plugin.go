package plugin

import (
	"bytes"
	"encoding/json"
	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/connections"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

var (
	OperationDefinitions = map[string]*handlers.OperationDefinition{}
	requestUrl           string
)

type openApiPlugin struct {
	actions     []plugin.Action
	description plugin.Description
	openApiFile string
}

type actionOutput struct {
	Output string `json:"output"`
	Error  string `json:"error"`
}

func (p *openApiPlugin) Describe() plugin.Description {
	log.Debug("Handling Describe request!")
	return p.description
}

func (p *openApiPlugin) GetActions() []plugin.Action {
	log.Debug("Handling GetActions request!")
	return p.actions
}

func (p *openApiPlugin) TestCredentials(map[string]connections.ConnectionInstance) (*plugin.CredentialsValidationResponse, error) {
	// ToDo: replace with real implementation
	return &plugin.CredentialsValidationResponse{
		AreCredentialsValid:   true,
		RawValidationResponse: nil,
	}, nil
}

func (p *openApiPlugin) ExecuteAction(actionContext *plugin.ActionContext, request *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error) {
	client := &http.Client{}
	openApiRequest, err := p.parseActionRequest(actionContext, request)

	if err != nil {
		return nil, err
	}

	if err = p.setAuthenticationHeaders(actionContext, openApiRequest); err != nil {
		return nil, err
	}

	response, err := client.Do(openApiRequest)

	if err != nil {
		return nil, err
	}

	result, err := buildResponse(response)

	if err != nil {
		return nil, err
	}

	return &plugin.ExecuteActionResponse{ErrorCode: 0, Result: result}, nil
}

func (p *openApiPlugin) parseActionRequest(actionContext *plugin.ActionContext, executeActionRequest *plugin.ExecuteActionRequest) (*http.Request, error) {
	operation := OperationDefinitions[executeActionRequest.Name]
	requestParameters, err := executeActionRequest.GetParameters()

	if err != nil {
		return nil, err
	}

	requestUrl = p.getRequestUrl(actionContext)
	requestPath := parsePathParams(requestParameters, operation, operation.Path)
	operationUrl, err := url.Parse(requestUrl + requestPath)

	if err != nil {
		return nil, err
	}

	requestBody, err := parseBodyParams(requestParameters, operation)

	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(operation.Method, operationUrl.String(), bytes.NewBuffer(requestBody))

	if err != nil {
		return nil, err
	}

	if operation.Method == consts.MethodPost {
		request.Header.Set(consts.ContentTypeHeader, consts.RequestBodyType)
	}

	parseHeaderParams(requestParameters, operation, request)
	parseCookieParams(requestParameters, operation, request)

	return request, nil
}

func NewOpenApiPlugin(name string, provider string, tags []string, connectionTypes map[string]connections.Connection, openApiFile string) (*openApiPlugin, error) {
	var actions []plugin.Action

	openApi, err := loadOpenApi(openApiFile)

	if err != nil {
		return nil, err
	}

	if len(openApi.Servers) == 0 {
		return nil, errors.New("no server URL provided in OpenApi file")
	}

	// Set default openApi server
	openApiServer := openApi.Servers[0]
	requestUrl = openApiServer.URL

	for urlVariableName, urlVariable := range openApiServer.Variables {
		requestUrl = strings.ReplaceAll(requestUrl, consts.ParamPrefix+urlVariableName+consts.ParamSuffix, urlVariable.Default)
	}

	err = handlers.DefineOperations(openApi)

	if err != nil {
		return nil, err
	}

	for _, operation := range OperationDefinitions {
		action := plugin.Action{
			Name:        operation.OperationId,
			Description: operation.Summary,
			Enabled:     true,
			EntryPoint:  operation.Path,
			Parameters:  map[string]plugin.ActionParameter{},
		}

		for _, pathParam := range operation.AllParams() {
			paramType := pathParam.Spec.Schema.Value.Type
			paramDefault := getParamDefault(pathParam.Spec.Schema.Value.Default, paramType)
			paramPlaceholder := getParamPlaceholder(pathParam.Spec.Example, paramType)
			paramOptions := getParamOptions(pathParam.Spec.Schema.Value.Enum, &paramType)

			action.Parameters[pathParam.ParamName] = plugin.ActionParameter{
				Type:        paramType,
				Description: pathParam.Spec.Description,
				Placeholder: paramPlaceholder,
				Required:    pathParam.Required,
				Default:     paramDefault,
				Options:     paramOptions,
			}
		}

		for _, paramBody := range operation.Bodies {
			if strings.ToLower(paramBody.ContentType) == consts.RequestBodyType {
				handleBodyParams(paramBody.Schema.OApiSchema, "", &action)
				break
			}
		}

		actions = append(actions, action)
	}

	return &openApiPlugin{
		actions: actions,
		description: plugin.Description{
			Name:        name,
			Description: openApi.Info.Description,
			Tags:        tags,
			Connections: connectionTypes,
			Provider:    provider,
		},
		openApiFile: openApiFile,
	}, nil
}

func loadOpenApi(filePath string) (openApi *openapi3.T, err error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	u, err := url.Parse(filePath)

	if err == nil && u.Scheme != "" && u.Host != "" {
		return loader.LoadFromURI(u)
	} else {
		return loader.LoadFromFile(filePath)
	}
}

func handleBodyParams(schema *openapi3.Schema, parentPath string, action *plugin.Action) {
	for propertyName, bodyProperty := range schema.Properties {
		fullParamPath := propertyName

		// Json params are represented as dot delimited params to allow proper parsing in UI later on
		if parentPath != "" {
			fullParamPath = parentPath + consts.BodyParamDelimiter + fullParamPath
		}

		// Keep recursion until leaf node is found
		if bodyProperty.Value.Properties != nil {
			handleBodyParams(bodyProperty.Value, fullParamPath, action)
		} else {
			paramType := bodyProperty.Value.Type
			paramOptions := getParamOptions(bodyProperty.Value.Enum, &paramType)
			paramPlaceholder := getParamPlaceholder(bodyProperty.Value.Example, paramType)
			paramDefault := getParamDefault(bodyProperty.Value.Default, paramType)
			isParamRequired := false

			for _, requiredParam := range schema.Required {
				if propertyName == requiredParam {
					isParamRequired = true
					break
				}
			}

			action.Parameters[fullParamPath] = plugin.ActionParameter{
				Type:        paramType,
				Description: bodyProperty.Value.Description,
				Placeholder: paramPlaceholder,
				Required:    isParamRequired,
				Default:     paramDefault,
				Options:     paramOptions,
			}
		}
	}
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
		paramDefault, _ = defaultValue.(string)

		return paramDefault
	}

	if defaultList, ok := defaultValue.([]interface{}); ok {
		var defaultStrings []string

		for _, value := range defaultList {
			valueString, _ := value.(string)
			defaultStrings = append(defaultStrings, valueString)
		}

		paramDefault = strings.Join(defaultStrings, consts.ArrayDelimiter)
	}

	return paramDefault
}

func buildResponse(response *http.Response) ([]byte, error) {
	var (
		responseOutput string
		responseError  string
	)

	defer func() {
		_ = response.Body.Close()
	}()

	result, err := ioutil.ReadAll(response.Body)

	if response.StatusCode != http.StatusOK {
		responseError = string(result)
	} else {
		responseOutput = string(result)
	}

	structuredOutput := actionOutput{
		Output: responseOutput,
		Error:  responseError,
	}

	parsedOutput, err := json.Marshal(structuredOutput)

	if err != nil {
		return nil, err
	}

	return parsedOutput, nil
}
