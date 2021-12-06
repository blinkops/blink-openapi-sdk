package plugin

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	customact "github.com/blinkops/blink-openapi-sdk/plugin/custom_actions"

	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	"github.com/blinkops/blink-openapi-sdk/zip"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/connections"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type (
	HeaderValuePrefixes  map[string]string
	HeaderAlias          map[string]string
	PathParams           []string
	JSONMap              interface{}
	SetCustomAuthHeaders func(connection map[string]interface{}, request *http.Request) error
	Result               struct {
		StatusCode int
		Body       []byte
	}
)

type openApiPlugin struct {
	actions             []plugin.Action
	description         plugin.Description
	requestUrl          string
	headerValuePrefixes HeaderValuePrefixes
	headerAlias         HeaderAlias
	mask                mask.Mask
	callbacks           Callbacks
}

type PluginMetadata struct {
	Name                string
	Provider            string
	MaskFile            string
	OpenApiFile         string
	Tags                []string
	HeaderValuePrefixes HeaderValuePrefixes
	HeaderAlias         HeaderAlias
}

type bodyMetadata struct {
	maskData   mask.Mask
	schemaPath string
	action     *plugin.Action
}

type parsedOpenApi struct {
	requestUrl  string
	description string
	actions     []plugin.Action
}

type Callbacks struct {
	TestCredentialsFunc  func(*plugin.ActionContext) (*plugin.CredentialsValidationResponse, error)
	ValidateResponse     func(Result) (bool, []byte)
	SetCustomAuthHeaders SetCustomAuthHeaders
	CustomActions        customact.CustomActions
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

func NewOpenApiPlugin(connectionTypes map[string]connections.Connection, meta PluginMetadata, callbacks Callbacks) (*openApiPlugin, error) {
	maskData, err := mask.ParseMask(meta.MaskFile)
	if err != nil {
		return nil, errors.Errorf("Cannot parse maskData file: %s", meta.MaskFile)
	}

	parsedFile, err := parseOpenApiFile(maskData, meta.OpenApiFile)
	if err != nil {
		return nil, err
	}

	// if no validate function was passed, the default one will be used
	if callbacks.ValidateResponse == nil {
		callbacks.ValidateResponse = validateDefault
	}

	var customActions []plugin.Action
	if len(callbacks.CustomActions.Actions) > 0 {
		customActions = callbacks.CustomActions.GetActions()
		if hasDuplicateActions(parsedFile.actions, customActions) {
			panic("One or more custom action has the same name as an openapi action")
		}
	}
	actions := append(customActions, parsedFile.actions...)

	return &openApiPlugin{
		actions:    actions,
		requestUrl: parsedFile.requestUrl,
		description: plugin.Description{
			Name:        meta.Name,
			Description: parsedFile.description,
			Tags:        meta.Tags,
			Connections: connectionTypes,
			Provider:    meta.Provider,
		},
		headerValuePrefixes: meta.HeaderValuePrefixes,
		headerAlias:         meta.HeaderAlias,
		mask:                maskData,
		callbacks:           callbacks,
	}, nil
}

func hasDuplicateActions(actions []plugin.Action, customActions []plugin.Action) bool {
	exists := make(map[string]bool)
	for _, act := range append(actions, customActions...) {
		if exists[act.Name] {
			return true
		}
		exists[act.Name] = true
	}
	return false
}

func isConnectionMandatory() bool {
	connectionNotMandatory, _ := strconv.ParseBool(os.Getenv(consts.ConnectionNotMandatory))
	return !connectionNotMandatory
}

func (p *openApiPlugin) ExecuteAction(actionContext *plugin.ActionContext, request *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error) {
	if p.callbacks.CustomActions.HasAction(request.Name) {
		return p.callbacks.CustomActions.Execute(actionContext, request)
	}
	connection, err := GetCredentials(actionContext, p.Describe().Provider)
	p.requestUrl = getRequestUrlFromConnection(p.requestUrl, connection)
	// Remove request url and leave only other authentication headers
	// We don't want to parse the URL with request params
	delete(connection, consts.RequestUrlKey)
	// Sometimes it's fine when there's no connection (like GitHub public repos) so we will not return an error

	if err != nil {
		if isConnectionMandatory() {
			return nil, err
		} else {
			log.Warn("No credentials provided")
		}
	}

	res := &plugin.ExecuteActionResponse{ErrorCode: consts.OK}
	openApiRequest, err := p.parseActionRequest(request)
	if err != nil {
		res.ErrorCode = consts.Error
		res.Result = []byte(err.Error())
		return res, nil
	}

	result, err := executeRequestWithCredentials(connection, openApiRequest, p.headerValuePrefixes, p.headerAlias, p.callbacks.SetCustomAuthHeaders, request.Timeout)

	res.Result = result.Body

	if err != nil {
		res.ErrorCode = consts.Error
		res.Result = []byte(err.Error())
		return res, nil
	}

	if valid, msg := p.callbacks.ValidateResponse(result); !valid {
		res.ErrorCode = consts.Error
		res.Result = msg
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
func ExecuteRequest(actionContext *plugin.ActionContext, httpRequest *http.Request, providerName string, headerValuePrefixes HeaderValuePrefixes, headerAlias HeaderAlias, timeout int32, setCustomHeaders SetCustomAuthHeaders) (Result, error) {
	connection, err := GetCredentials(actionContext, providerName)
	// Remove request url and leave only other authentication headers
	// We don't want to parse the URL with request params
	delete(connection, consts.RequestUrlKey)
	// Sometimes it's fine when there's no connection (like github public repos) so we will not return an error

	if err != nil {
		if isConnectionMandatory() {
			return Result{
				StatusCode: 0,
				Body:       nil,
			}, err
		} else {
			log.Warn("No credentials provided")
		}
	}

	return executeRequestWithCredentials(connection, httpRequest, headerValuePrefixes, headerAlias, setCustomHeaders, timeout)
}

func executeRequestWithCredentials(connection map[string]interface{}, httpRequest *http.Request, headerValuePrefixes HeaderValuePrefixes, headerAlias HeaderAlias, setCustomHeaders SetCustomAuthHeaders, timeout int32) (Result, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	result := Result{}
	log.Info(httpRequest.Method + ": " + httpRequest.URL.String())
	if setCustomHeaders != nil {
		if err := setCustomHeaders(connection, httpRequest); err != nil {
			log.Error(err)
			return result, fmt.Errorf("failed to set custom headers: %w", err)
		}
	} else if err := setAuthenticationHeaders(connection, httpRequest, headerValuePrefixes, headerAlias); err != nil {
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

func (p *openApiPlugin) parseActionRequest(executeActionRequest *plugin.ExecuteActionRequest) (*http.Request, error) {
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

	if operation.Method == http.MethodPost || operation.Method == http.MethodPut || operation.Method == http.MethodPatch {
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

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if strings.EqualFold(b, a) {
			return true
		}
	}
	return false
}

func parseOpenApiFile(maskData mask.Mask, OpenApiFile string) (parsedOpenApi, error) {
	var actions []plugin.Action

	openApi, err := loadOpenApi(OpenApiFile)
	if err != nil {
		return parsedOpenApi{}, err
	}

	if len(openApi.Servers) == 0 {
		return parsedOpenApi{}, err
	}

	// Set default openApi server
	openApiServer := openApi.Servers[0]
	requestUrl := openApiServer.URL

	for urlVariableName, urlVariable := range openApiServer.Variables {
		requestUrl = strings.ReplaceAll(requestUrl, consts.ParamPrefix+urlVariableName+consts.ParamSuffix, urlVariable.Default)
	}

	err = handlers.DefineOperations(openApi)

	if err != nil {
		return parsedOpenApi{}, err
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

				handleBodyParams(bodyMetadata{maskData, "", &action}, paramBody.Schema.OApiSchema, "", paramBody.Required)
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
	return parsedOpenApi{
		description: openApi.Info.Description,
		requestUrl:  requestUrl,
		actions:     actions,
	}, nil
}

func loadOpenApi(filePath string) (openApi *openapi3.T, err error) {
	const pathErrorRe = `open (.*):`
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	u, err := url.Parse(filePath)

	if err == nil && u.Scheme != "" && u.Host != "" {
		return loader.LoadFromURI(u)
	}

	if os.Getenv(consts.ENVStatusKey) != "" {
		for {
			// when running in prod the openAPI file is gzipped
			parsed, err := zip.LoadFromGzipFile(loader, filePath+consts.GzipFile)
			if err != nil {
				// loadFromGzipFile failed because it couldn't find a ref file
				// because the ref file is also gzipped.
				re := regexp.MustCompile(pathErrorRe)

				// find the path of the ref file in the error.
				refPath := re.FindStringSubmatch(err.Error())[1]

				if err = zip.UnzipFile(refPath); err != nil {
					return nil, err
				}

				// call the function again to continue parsing the openAPI file.
				continue
			}

			return parsed, nil
		}
	}
	// normal yaml
	return loader.LoadFromFile(filePath)
}

// GetRequestUrl Exported for plugin test credentials function
func GetRequestUrl(actionContext *plugin.ActionContext, provider string) (string, error) {
	if connection, err := GetCredentials(actionContext, provider); err != nil {
		log.Errorf("Failed to fetch credentials for %s, got: %v", provider, err)
		return "", errors.Errorf("Failed to fetch the request URL for %s", provider)
	} else {
		return getRequestUrlFromConnection("", connection), nil
	}
}

func validateDefault(response Result) (bool, []byte) {
	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		return true, nil
	}
	return false, response.Body
}
