package plugin

import (
	"fmt"
	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/connections"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type HeaderValuePrefixes map[string]string
type HeaderAlias map[string]string
type PathParams []string
type JSONMap interface{}
type GetTokenFromCredentials func(map[string]interface{}) (string, error)
type Result struct {
	StatusCode int
	Body       []byte
}

type openApiPlugin struct {
	actions     []plugin.Action
	description         plugin.Description
	requestUrl          string
	headerValuePrefixes HeaderValuePrefixes
	headerAlias         HeaderAlias
	pathParams          PathParams
	mask      mask.Mask
	callbacks PluginChecks
}

type PluginMetadata struct {
	Name                string
	Provider            string
	MaskFile            string
	OpenApiFile         string
	Tags                []string
	HeaderValuePrefixes HeaderValuePrefixes
	HeaderAlias         HeaderAlias
	PathParams          PathParams
}

type parseOpenApiResponse struct {
	requestUrl string
	description string
	actions []plugin.Action
}

type PluginChecks struct {
	TestCredentialsFunc func(*plugin.ActionContext) (*plugin.CredentialsValidationResponse, error)
	ValidateResponse    func(Result) (bool, []byte)
	GetTokenFromCrendentials GetTokenFromCredentials
}

func (p *openApiPlugin) Describe() plugin.Description {
	log.Debug("Handling Describe request!")
	return p.description
}

func (p *openApiPlugin) GetActions() []plugin.Action {
	log.Debug("Handling GetActions request!")
	return p.actions
}

func (p *openApiPlugin) TestCredentials(conn map[string]*connections.ConnectionInstance) (*plugin.CredentialsValidationResponse, error) {
	return p.callbacks.TestCredentialsFunc(plugin.NewActionContext(nil, conn))

}

func (p *openApiPlugin) actionExist(actionName string) bool {
	for _, val := range p.actions {
		if val.Name == actionName {
			return true
		}
	}
	return false
}

func (p *openApiPlugin) ExecuteAction(actionContext *plugin.ActionContext, request *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error) {
	connection, err := getCredentials(actionContext, p.Describe().Provider)
	p.requestUrl = getRequestUrlFromConnection(p.requestUrl, connection)
	// Remove request url and leave only other authentication headers
	// We don't want to parse the URL with request params
	delete(connection, consts.RequestUrlKey)
	// Sometimes it's fine when there's no connection (like github public repos) so we will not return an error
	if err != nil {
		log.Warn("No credentials provided")
	}

	res := &plugin.ExecuteActionResponse{ErrorCode: consts.OK}
	openApiRequest, err := p.parseActionRequest(connection, request)

	if err != nil {
		res.ErrorCode = consts.Error
		res.Result = []byte(err.Error())
		return res, nil
	}

	result, err := executeRequestWithCredentials(connection, openApiRequest, p.headerValuePrefixes, p.headerAlias, p.callbacks.GetTokenFromCrendentials, request.Timeout)
	res.Result = result.Body

	if err != nil {
		res.ErrorCode = consts.Error
		res.Result = []byte(err.Error())
		return res, nil
	}

	// if no validate response function was passed no response check will occur.
	if p.callbacks.ValidateResponse != nil && len(result.Body) > 0 {

		if valid, msg := p.callbacks.ValidateResponse(result); !valid {
			res.ErrorCode = consts.Error
			res.Result = msg
		}
	}

	return res, nil
}

func fixRequestURL(r *http.Request) error {

	if r.URL.Scheme == "" {
		r.URL.Scheme = "https"
	}
	val, err := url.Parse(strings.TrimSuffix(r.URL.String(), "/"))
	r.URL = val

	return err
}

// ExecuteRequest is used by the 'validate' method in most openapi plugins.
func ExecuteRequest(actionContext *plugin.ActionContext, httpRequest *http.Request, providerName string, headerValuePrefixes HeaderValuePrefixes, headerAlias HeaderAlias, timeout int32, manipulateCredentials GetTokenFromCredentials) (Result, error) {
	connection, err := getCredentials(actionContext, providerName)
	// Remove request url and leave only other authentication headers
	// We don't want to parse the URL with request params
	delete(connection, consts.RequestUrlKey)
	// Sometimes it's fine when there's no connection (like github public repos) so we will not return an error
	if err != nil {
		log.Warn("No credentials provided")
	}

	return executeRequestWithCredentials(connection, httpRequest, headerValuePrefixes, headerAlias, manipulateCredentials, timeout)
}

func executeRequestWithCredentials(connection map[string]interface{}, httpRequest *http.Request, headerValuePrefixes HeaderValuePrefixes, headerAlias HeaderAlias, manipulateCredentials GetTokenFromCredentials, timeout int32) (Result, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	result := Result{}
	log.Info(httpRequest.URL)
	if err := setAuthenticationHeaders(connection, httpRequest, manipulateCredentials, headerValuePrefixes, headerAlias); err != nil {
		log.Error(err)
		return result, err
	}

	if err := fixRequestURL(httpRequest); err != nil {
		log.Error(err)
		return result, err
	}

	response, err := client.Do(httpRequest)

	if err != nil {
		log.Error(err)
		return result, err
	}
	// closing the response body, not closing can cause a mem leak
	defer func() {
		if err = response.Body.Close(); err != nil {
			log.Error(err)
		}
	}()

	result.Body, err = ioutil.ReadAll(response.Body)
	result.StatusCode = response.StatusCode

	log.Debug(result.Body)
	log.Info(result.StatusCode)

	return result, err
}

func (p *openApiPlugin) parseActionRequest(connection map[string]interface{}, executeActionRequest *plugin.ExecuteActionRequest) (*http.Request, error) {
	actionName := executeActionRequest.Name

	if !p.actionExist(actionName) {
		err := errors.New("No such method")
		log.Error(err)
		return nil, err
	}

	actionName = p.mask.ReplaceActionAlias(actionName)
	operation := handlers.OperationDefinitions[actionName]

	// get the parameters from the request.
	rawParameters, err := executeActionRequest.GetParameters()

	if err != nil {
		return nil, err
	}

	// replace the raw parameters with their alias.
	requestParameters := p.mask.ReplaceActionParametersAliases(actionName, rawParameters)

	// add to request parameters
	paramsFromConnection := getPathParamsFromConnection(connection, p.pathParams)

	for k, v := range paramsFromConnection {
		requestParameters[k] = v
	}

	requestPath := parsePathParams(requestParameters, operation, operation.Path)
	operationUrl, err := url.Parse(p.requestUrl + requestPath)

	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(operation.Method, operationUrl.String(), nil)

	if err != nil {
		return nil, err
	}

	if operation.Method != http.MethodGet {
		err = parseBodyParams(requestParameters, operation, request)
		if err != nil {
			return nil, err
		}
	}

	if operation.Method == http.MethodPost || operation.Method == http.MethodPut {
		bodyType := operation.GetDefaultBodyType()

		if bodyType != "" {
			request.Header.Set(consts.ContentTypeHeader, bodyType)
		}
	}

	parseHeaderParams(requestParameters, operation, request)
	parseCookieParams(requestParameters, operation, request)
	parseQueryParams(requestParameters, operation, request)

	return request, nil
}

func getPathParamsFromConnection(connection map[string]interface{}, params PathParams) (map[string]string) {
	paramsFromConnection := map[string]string{}
	for header, headerValue := range connection {
		if headerValueString, ok := headerValue.(string); ok {
			if StringInSlice(header, params) {
				paramsFromConnection[header] = headerValueString
			}

		}

	}
	return paramsFromConnection
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if strings.EqualFold(b, a) {
			return true
		}
	}
	return false
}

func NewOpenApiPlugin(connectionTypes map[string]connections.Connection, meta PluginMetadata, checks PluginChecks) (*openApiPlugin, error) {

	mask, err := mask.ParseMask(meta.MaskFile)

	if err != nil {
		return nil, errors.Errorf("Cannot parse mask file: %s", meta.MaskFile)
	}

	parseOpenApiResponse, err := parseOpenApiFile(mask, meta.OpenApiFile)
	if err != nil {
		return nil, err
	}

	return &openApiPlugin{
		actions:             parseOpenApiResponse.actions,
		requestUrl: parseOpenApiResponse.requestUrl,
		description: plugin.Description{
			Name:        meta.Name,
			Description: parseOpenApiResponse.description,
			Tags:        meta.Tags,
			Connections: connectionTypes,
			Provider:    meta.Provider,
		},
		headerValuePrefixes: meta.HeaderValuePrefixes,
		headerAlias:         meta.HeaderAlias,
		mask:       mask,
		callbacks:  checks,
	}, nil
}

func parseOpenApiFile(maskData mask.Mask, OpenApiFile string) (parseOpenApiResponse, error) {
	var actions []plugin.Action

	openApi, err := loadOpenApi(OpenApiFile)

	if err != nil {
		return parseOpenApiResponse{}, err
	}

	if len(openApi.Servers) == 0 {
		return parseOpenApiResponse{}, err
	}

	// Set default openApi server
	openApiServer := openApi.Servers[0]
	requestUrl := openApiServer.URL

	for urlVariableName, urlVariable := range openApiServer.Variables {
		requestUrl = strings.ReplaceAll(requestUrl, consts.ParamPrefix+urlVariableName+consts.ParamSuffix, urlVariable.Default)
	}

	err = handlers.DefineOperations(openApi)

	if err != nil {
		return parseOpenApiResponse{}, err
	}

	for _, operation := range handlers.OperationDefinitions {
		actionName := operation.OperationId

		// Skip masked actions
		if maskData.Actions != nil {
			if maskedAction := maskData.GetAction(actionName); maskedAction == nil {
				continue
			} else {
				if maskedAction.Alias != "" {
					actionName = maskedAction.Alias
				}
			}
		}

		action := plugin.Action{
			Name:        actionName,
			Description: operation.Summary,
			Enabled:     true,
			EntryPoint:  operation.Path,
			Parameters:  map[string]plugin.ActionParameter{},
		}

		for _, pathParam := range operation.AllParams() {
			paramName := pathParam.ParamName
			paramDescription := pathParam.Schema.Description

			if paramDescription == "" {
				paramDescription = pathParam.Spec.Description
			}

			if actionParam := parseActionParam(maskData, action.Name, &paramName, pathParam.Spec.Schema, pathParam.Required, paramDescription); actionParam != nil {
				action.Parameters[paramName] = *actionParam
			}
		}

		for _, paramBody := range operation.Bodies {
			if paramBody.DefaultBody {
				handleBodyParams(maskData, paramBody.Schema.OApiSchema, "", &action)
				break
			}
		}

		actions = append(actions, action)
	}

	// sort the actions
	// each time we parse the openapi the actions are added to the map in different order.
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name < actions[j].Name
	})
	return parseOpenApiResponse{
		description: openApi.Info.Description,
		requestUrl: requestUrl,
		actions: actions,
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

func handleBodyParams(maskData mask.Mask, schema *openapi3.Schema, parentPath string, action *plugin.Action) {
	handleBodyParamOfType(maskData, schema, parentPath, action)

	for propertyName, bodyProperty := range schema.Properties {
		fullParamPath := propertyName

		if hasDuplicates(parentPath + consts.BodyParamDelimiter + fullParamPath) {
			continue
		}

		// Json params are represented as dot delimited params to allow proper parsing in UI later on
		if parentPath != "" {
			fullParamPath = parentPath + consts.BodyParamDelimiter + fullParamPath
		}

		// Keep recursion until leaf node is found
		if bodyProperty.Value.Properties != nil {
			handleBodyParams(maskData, bodyProperty.Value, fullParamPath, action)
		} else {
			handleBodyParamOfType(maskData, bodyProperty.Value, fullParamPath, action)
			isParamRequired := false

			for _, requiredParam := range schema.Required {
				if propertyName == requiredParam {
					isParamRequired = true
					break
				}
			}

			if actionParam := parseActionParam(maskData, action.Name, &fullParamPath, bodyProperty, isParamRequired, bodyProperty.Value.Description); actionParam != nil {
				action.Parameters[fullParamPath] = *actionParam
			}
		}
	}
}

func handleBodyParamOfType(maskData mask.Mask, schema *openapi3.Schema, parentPath string, action *plugin.Action) {
	if schema.AllOf != nil || schema.AnyOf != nil || schema.OneOf != nil {

		allSchemas := []openapi3.SchemaRefs{schema.AllOf, schema.AnyOf, schema.OneOf}

		// find properties nested in Allof, Anyof, Oneof
		for _, schemaType := range allSchemas {
			for _, schemaParams := range schemaType {
				handleBodyParams(maskData, schemaParams.Value, parentPath, action)
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

func hasDuplicates(path string) bool {
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

func extractTypeFromFormat(paramFormat string) string {
	paramType := strings.Split(paramFormat, mask.FormatDelimiter)[0]

	for _, prefixType := range mask.FormatPrefixes {
		if paramType == prefixType {
			return paramType
		}
	}

	return ""
}
