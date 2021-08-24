package plugin

import (
	"encoding/json"
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
	"strings"
	"time"
)

var (
	requestUrl string
)

type HeaderPrefixes map[string]string

type JSONMap interface{}

type Result struct {
	StatusCode int
	Body []byte
}

type openApiPlugin struct {
	actions             []plugin.Action
	description         plugin.Description
	openApiFile         string
	TestCredentialsFunc func(ctx *plugin.ActionContext) (*plugin.CredentialsValidationResponse, error)
	ValidateResponse    func(JSONMap) (bool, []byte)
	HeaderPrefixes      HeaderPrefixes
}

type PluginMetadata struct {
	Name           string
	Provider       string
	MaskFile       string
	OpenApiFile    string
	Tags           []string
	HeaderPrefixes HeaderPrefixes
}

type PluginChecks struct {
	TestCredentialsFunc func(ctx *plugin.ActionContext) (*plugin.CredentialsValidationResponse, error)
	ValidateResponse    func(JSONMap JSONMap) (bool, []byte)
}

func (p *openApiPlugin) Describe() plugin.Description {
	log.Debug("Handling Describe request!")
	return p.description
}

func (p *openApiPlugin) GetActions() []plugin.Action {
	log.Debug("Handling GetActions request!")
	return p.actions
}

func (p *openApiPlugin) TestCredentials(conn map[string]connections.ConnectionInstance) (*plugin.CredentialsValidationResponse, error) {

	return p.TestCredentialsFunc(plugin.NewActionContext(nil, conn))

}

func (p *openApiPlugin) ExecuteAction(actionContext *plugin.ActionContext, request *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error) {
	res := &plugin.ExecuteActionResponse{ErrorCode: consts.OK}
	openApiRequest, err := p.parseActionRequest(actionContext, request)

	if err != nil {
		res.ErrorCode = consts.Error
		res.Result = []byte(err.Error())
		return res, nil
	}

	result, err := ExecuteRequest(actionContext, openApiRequest, p.Describe().Provider, p.HeaderPrefixes, request.Timeout)
	res.Result = result.Body

	if err != nil {
		res.ErrorCode = consts.Error
		res.Result = []byte(err.Error())
		return res, nil
	}

	// if no validate response function was passed no response check will occur.
	if p.ValidateResponse != nil {
		var data JSONMap

		//check if result.Body is empty
		if len(result.Body) > 0 {
			if err = json.Unmarshal(result.Body, &data); err != nil {
				res.ErrorCode = consts.Error
				res.Result = []byte(err.Error())
				return res, nil
			}

			if valid, msg := p.ValidateResponse(data); !valid {
				res.ErrorCode = consts.Error
				res.Result = msg
			}
		}


	}

	return res, nil
}

func FixRequestURL(r *http.Request) error {

	r.URL.Scheme = "https"

	val, err := url.Parse(r.URL.String())
	r.URL = val

	return err
}

func ExecuteRequest(actionContext *plugin.ActionContext, httpRequest *http.Request, providerName string, HeaderPrefixes map[string]string, timeout int32) (Result, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	result := Result{}

	if err := SetAuthenticationHeaders(actionContext, httpRequest, providerName, HeaderPrefixes); err != nil {
		return result, err
	}

	if err := FixRequestURL(httpRequest); err != nil {
		return result, err
	}

	response, err := client.Do(httpRequest)

	if err != nil {
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

	return result, err
}

func (p *openApiPlugin) parseActionRequest(actionContext *plugin.ActionContext, executeActionRequest *plugin.ExecuteActionRequest) (*http.Request, error) {
	actionName := executeActionRequest.Name
	actionName = mask.ReplaceActionAlias(actionName)

	operation := handlers.OperationDefinitions[actionName]

	// get the parameters from the request.
	rawParameters, err := executeActionRequest.GetParameters()

	if err != nil {
		return nil, err
	}

	// replace the raw parameters with their alias.
	requestParameters := mask.ReplaceActionParametersAliases(actionName, rawParameters)

	requestUrl = GetRequestUrl(actionContext, p.Describe().Provider)

	requestPath := parsePathParams(requestParameters, operation, operation.Path)

	operationUrl, err := url.Parse(requestUrl + requestPath)

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

	if operation.Method == http.MethodPost {
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

func NewOpenApiPlugin(connectionTypes map[string]connections.Connection, meta PluginMetadata, checks PluginChecks) (*openApiPlugin, error) {

	if checks.TestCredentialsFunc == nil {
		panic("TestCredentials function is missing")
	}

	var actions []plugin.Action

	openApi, err := loadOpenApi(meta.OpenApiFile)

	if err != nil {
		return nil, err
	}

	err = mask.ParseMask(meta.MaskFile)

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

	for _, operation := range handlers.OperationDefinitions {
		actionName := operation.OperationId

		// Skip masked actions
		if mask.MaskData != nil {
			if maskedAction := mask.MaskData.GetAction(actionName); maskedAction == nil {
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
			paramType := pathParam.Spec.Schema.Value.Type
			paramDefault := getParamDefault(pathParam.Spec.Schema.Value.Default, paramType)
			paramPlaceholder := getParamPlaceholder(pathParam.Spec.Example, paramType)
			paramOptions := getParamOptions(pathParam.Spec.Schema.Value.Enum, &paramType)
			isParamRequired := pathParam.Required

			if mask.MaskData != nil {
				if maskedParam := mask.MaskData.GetParameter(actionName, paramName); maskedParam == nil {
					continue
				} else {
					if maskedParam.Alias != "" {
						paramName = maskedParam.Alias
					}
				}
			}

			action.Parameters[paramName] = plugin.ActionParameter{
				Type:        paramType,
				Description: pathParam.Spec.Description,
				Placeholder: paramPlaceholder,
				Required:    isParamRequired,
				Default:     paramDefault,
				Options:     paramOptions,
			}
		}

		for _, paramBody := range operation.Bodies {
			if paramBody.DefaultBody {
				handleBodyParams(paramBody.Schema.OApiSchema, "", &action)
				break
			}
		}

		actions = append(actions, action)
	}

	return &openApiPlugin{
		TestCredentialsFunc: checks.TestCredentialsFunc,
		ValidateResponse:    checks.ValidateResponse,
		actions:             actions,
		HeaderPrefixes:      meta.HeaderPrefixes,
		description: plugin.Description{
			Name:        meta.Name,
			Description: openApi.Info.Description,
			Tags:        meta.Tags,
			Connections: connectionTypes,
			Provider:    meta.Provider,
		},
		openApiFile: meta.OpenApiFile,
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
		} else if bodyProperty.Value.AllOf != nil || bodyProperty.Value.AnyOf != nil || bodyProperty.Value.OneOf != nil {

			var a []openapi3.SchemaRefs

			a = append(a, bodyProperty.Value.AllOf, bodyProperty.Value.AnyOf, bodyProperty.Value.OneOf)

			for _, v := range a {
				for _, v2  := range v {
					handleBodyParams(v2.Value, fullParamPath, action)
				}
			}

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

			if mask.MaskData != nil {
				if maskedParam := mask.MaskData.GetParameter(action.Name, fullParamPath); maskedParam == nil {
					continue
				} else {
					if maskedParam.Alias != "" {
						fullParamPath = maskedParam.Alias
					}
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
