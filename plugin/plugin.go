package plugin

import (
	"bytes"
	"encoding/json"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/connections"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	typeArray              = "array"
	typeInteger            = "integer"
	typeBoolean            = "boolean"
	typeObject             = "object"
	bodyParamDelimiter     = "."
	requestBodyType        = "application/json"
	paramPrefix            = "{"
	paramSuffix            = "}"
	requestUrlKey          = "REQUEST_URL"
	arrayDelimiter         = ","
	contentTypeHeader      = "Content-Type"
	methodPost             = "POST"
	paramPlaceholderPrefix = "Example: "
)

var (
	operationDefinitions = map[string]*operationDefinition{}
	requestUrl           string
)

type openApiPlugin struct {
	actions     []plugin.Action
	description plugin.Description
	openApiFile string
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
	result := []byte{}
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

	if _, err = response.Body.Read(result); err != nil {
		return nil, err
	}

	return &plugin.ExecuteActionResponse{ErrorCode: int64(response.StatusCode), Result: result}, nil
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
		requestUrl = strings.ReplaceAll(requestUrl, paramPrefix+urlVariableName+paramSuffix, urlVariable.Default)
	}

	err = defineOperations(openApi)

	if err != nil {
		return nil, err
	}

	for _, operation := range operationDefinitions {
		action := plugin.Action{
			Name:        operation.operationId,
			Description: operation.summary,
			Enabled:     true,
			EntryPoint:  operation.path,
			Parameters:  map[string]plugin.ActionParameter{},
		}

		for _, pathParam := range operation.allParams() {
			var paramPlaceholder string
			paramType := pathParam.spec.Schema.Value.Type
			paramOptions := []string{}
			paramDefault, _ := pathParam.spec.Schema.Value.Default.(string)

			// Get Parameter dropdown options
			for _, option := range pathParam.spec.Schema.Value.Items.Value.Enum {
				if optionString, ok := option.(string); ok {
					paramOptions = append(paramOptions, optionString)
				}
			}

			if paramType != typeObject {
				paramPlaceholder, _ = pathParam.spec.Example.(string)

				if paramPlaceholder != "" {
					paramPlaceholder = paramPlaceholderPrefix + paramPlaceholder
				}
			}

			action.Parameters[pathParam.paramName] = plugin.ActionParameter{
				Type:        paramType,
				Description: pathParam.spec.Description,
				Placeholder: paramPlaceholder,
				Required:    pathParam.required,
				Default:     paramDefault,
				Options:     paramOptions,
			}
		}

		for _, paramBody := range operation.bodies {
			if strings.ToLower(paramBody.contentType) == requestBodyType {
				handleBodyParams(paramBody.schema.oApiSchema, "", &action)
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

func handleBodyParams(schema *openapi3.Schema, parentPath string, action *plugin.Action) {
	for propertyName, bodyProperty := range schema.Properties {
		fullParamPath := propertyName

		// Json params are represented as dot delimited params to allow proper parsing in UI later on
		if parentPath != "" {
			fullParamPath = parentPath + bodyParamDelimiter + fullParamPath
		}

		// Keep recursion until leaf node is found
		if bodyProperty.Value.Properties != nil {
			handleBodyParams(bodyProperty.Value, fullParamPath, action)
		} else {
			var paramPlaceholder string
			paramType := bodyProperty.Value.Type
			paramOptions := []string{}
			paramDefault, _ := bodyProperty.Value.Default.(string)
			isParamRequired := false

			for _, requiredParam := range schema.Required {
				if propertyName == requiredParam {
					isParamRequired = true
					break
				}
			}

			// Get Parameter dropdown options
			for _, option := range bodyProperty.Value.Enum {
				if optionString, ok := option.(string); ok {
					paramOptions = append(paramOptions, optionString)
				}
			}

			if paramType != typeObject {
				paramPlaceholder, _ = bodyProperty.Value.Example.(string)

				if paramPlaceholder != "" {
					paramPlaceholder = paramPlaceholder + paramPlaceholder
				}
			}

			action.Parameters[fullParamPath] = plugin.ActionParameter{
				Type:        paramType,
				Description: bodyProperty.Value.Description,
				Required:    isParamRequired,
				Default:     paramDefault,
				Options:     paramOptions,
			}
		}
	}
}

func (p *openApiPlugin) parseActionRequest(actionContext *plugin.ActionContext, executeActionRequest *plugin.ExecuteActionRequest) (*http.Request, error) {
	operation := operationDefinitions[executeActionRequest.Name]
	requestParameters, err := executeActionRequest.GetParameters()

	if err != nil {
		return nil, err
	}

	requestUrl = p.getRequestUrl(actionContext)
	requestPath := parsePathParams(requestParameters, operation, operation.path)
	operationUrl, err := url.Parse(requestUrl + requestPath)

	if err != nil {
		return nil, err
	}

	requestBody, err := parseBodyParams(requestParameters, operation)

	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(operation.method, operationUrl.String(), bytes.NewBuffer(requestBody))

	if err != nil {
		return nil, err
	}

	if operation.method == methodPost {
		request.Header.Set(contentTypeHeader, requestBodyType)
	}

	parseHeaderParams(requestParameters, operation, request)
	parseCookieParams(requestParameters, operation, request)

	return request, nil
}

func parseCookieParams(requestParameters map[string]string, operation *operationDefinition, request *http.Request) {
	for paramName, paramValue := range requestParameters {
		for _, cookieParam := range operation.cookieParams {
			if paramName == cookieParam.paramName {
				cookie := &http.Cookie{
					Name:  paramName,
					Value: paramValue,
				}

				request.AddCookie(cookie)
			}
		}
	}
}

func parseHeaderParams(requestParameters map[string]string, operation *operationDefinition, request *http.Request) {
	for paramName, paramValue := range requestParameters {
		for _, headerParam := range operation.headerParams {
			if paramName == headerParam.paramName {
				request.Header.Set(paramName, paramValue)
			}
		}
	}
}

func parsePathParams(requestParameters map[string]string, operation *operationDefinition, path string) string {
	requestPath := path

	for paramName, paramValue := range requestParameters {
		for _, pathParam := range operation.pathParams {
			if paramName == pathParam.paramName {
				requestPath = strings.ReplaceAll(path, paramPrefix+paramName+paramSuffix, paramValue)
			}
		}
	}

	return requestPath
}

func parseBodyParams(requestParameters map[string]string, operation *operationDefinition) ([]byte, error) {
	requestBody := map[string]interface{}{}
	operationBody := &requestBodyDefinition{}

	// Looking for a json type body schema
	for _, paramBody := range operation.bodies {
		if strings.ToLower(paramBody.contentType) == requestBodyType {
			operationBody = &paramBody
			break
		}
	}

	// Add "." delimited params as request body
	for paramName, paramValue := range requestParameters {
		if strings.Contains(paramName, bodyParamDelimiter) {
			mapKeys := strings.Split(paramName, bodyParamDelimiter)
			buildRequestBody(mapKeys, operationBody.schema.oApiSchema, paramValue, requestBody)
		}
	}

	marshaledBody, err := json.Marshal(requestBody)

	if err != nil {
		return nil, err
	}

	return marshaledBody, nil
}

// Build nested json request body from "." delimited parameters
func buildRequestBody(mapKeys []string, propertySchema *openapi3.Schema, paramValue string, requestBody map[string]interface{}) {
	key := mapKeys[0]

	// Keep recursion going until leaf node is found
	if len(mapKeys) == 1 {
		subPropertySchema := getPropertyByName(key, propertySchema)

		if subPropertySchema != nil {
			requestBody[mapKeys[len(mapKeys)-1]] = castBodyParamType(paramValue, subPropertySchema.Type)
		} else {
			log.Errorf("Invalid request body param passed: %s", key)
		}

	} else {
		if _, ok := requestBody[key]; !ok {
			requestBody[key] = map[string]interface{}{}
		}

		subPropertySchema := getPropertyByName(key, propertySchema)
		buildRequestBody(mapKeys[1:], subPropertySchema, paramValue, requestBody[key].(map[string]interface{}))
	}
}

// Cast proper parameter types when building json request body
func castBodyParamType(paramValue string, paramType string) interface{} {
	switch paramType {
	case typeInteger:
		if intValue, err := strconv.Atoi(paramValue); err != nil {
			return paramValue
		} else {
			return intValue
		}
	case typeBoolean:
		if boolValue, err := strconv.ParseBool(paramValue); err != nil {
			return paramValue
		} else {
			return boolValue
		}
	case typeArray:
		return strings.Split(paramValue, arrayDelimiter)
	default:
		return paramValue
	}
}

// Credentials should be saved as headerName -> value according to the api definition
func (p *openApiPlugin) setAuthenticationHeaders(actionContext *plugin.ActionContext, request *http.Request) error {
	securityHeaders, err := p.getCredentials(actionContext)

	if err != nil {
		return err
	}

	for header, headerValue := range securityHeaders {
		if headerValueString, ok := headerValue.(string); ok {
			request.Header.Set(strings.ToUpper(header), headerValueString)
		}
	}

	return nil
}

func (p *openApiPlugin) getRequestUrl(actionContext *plugin.ActionContext) string {
	connection, err := actionContext.GetCredentials(p.Describe().Provider)

	if err != nil {
		return requestUrl
	}

	if explicitRequestUrl, ok := connection[requestUrlKey].(string); ok {
		return explicitRequestUrl
	}

	return requestUrl
}

func (p *openApiPlugin) getCredentials(actionContext *plugin.ActionContext) (map[string]interface{}, error) {
	connection, err := actionContext.GetCredentials(p.Describe().Provider)

	if err != nil {
		return nil, err
	}

	// Remove request url and leave only other authentication headers
	delete(connection, requestUrlKey)

	return connection, nil
}
