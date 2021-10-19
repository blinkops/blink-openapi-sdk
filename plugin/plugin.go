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
	"github.com/urfave/cli/v2"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

var (
	requestUrl string
)

type HeaderValuePrefixes map[string]string
type HeaderAlias map[string]string
type PathParams []string

type JSONMap interface{}

type Result struct {
	StatusCode int
	Body       []byte
}

type OpenApiPlugin struct {
	actions     []plugin.Action
	description plugin.Description

	HeaderValuePrefixes HeaderValuePrefixes
	HeaderAlias         HeaderAlias
	PathParams          PathParams
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

var helpingFunctions PluginChecks

type PluginChecks struct {
	TestCredentialsFunc func(ctx *plugin.ActionContext) (*plugin.CredentialsValidationResponse, error)
	ValidateResponse    func(Result) (bool, []byte)
}

func (p *OpenApiPlugin) Describe() plugin.Description {
	log.Debug("Handling Describe request!")
	return p.description
}

func (p *OpenApiPlugin) GetActions() []plugin.Action {
	log.Debug("Handling GetActions request!")
	return p.actions
}

func (p *OpenApiPlugin) TestCredentials(conn map[string]connections.ConnectionInstance) (*plugin.CredentialsValidationResponse, error) {

	return helpingFunctions.TestCredentialsFunc(plugin.NewActionContext(nil, conn))

}

func (p *OpenApiPlugin) ActionExist(actionName string) bool {
	for _, val := range p.actions {
		if val.Name == actionName {
			return true
		}
	}
	return false
}


func (p *OpenApiPlugin) ExecuteAction(actionContext *plugin.ActionContext, request *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error) {
	connection, err := GetCredentials(actionContext, p.Describe().Provider)

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

	result, err := ExecuteRequestWithCredentials(connection, openApiRequest, p.HeaderValuePrefixes, p.HeaderAlias, request.Timeout)
	res.Result = result.Body

	if err != nil {
		res.ErrorCode = consts.Error
		res.Result = []byte(err.Error())
		return res, nil
	}

	// if no validate response function was passed no response check will occur.
	if helpingFunctions.ValidateResponse != nil && len(result.Body) > 0 {

		if valid, msg := helpingFunctions.ValidateResponse(result); !valid {
			res.ErrorCode = consts.Error
			res.Result = msg
		}
	}

	return res, nil
}

func FixRequestURL(r *http.Request) error {

	if r.URL.Scheme == "" {
		r.URL.Scheme = "https"
	}
	val, err := url.Parse(strings.TrimSuffix(r.URL.String(), "/"))
	r.URL = val

	return err
}

func ExecuteRequest(actionContext *plugin.ActionContext, httpRequest *http.Request, providerName string, headerValuePrefixes HeaderValuePrefixes, headerAlias HeaderAlias, timeout int32) (Result, error) {
	connection, err := GetCredentials(actionContext, providerName)

	if err != nil {
		log.Warn("No credentials provided")
	}

	return ExecuteRequestWithCredentials(connection, httpRequest, headerValuePrefixes, headerAlias, timeout)
}

func ExecuteRequestWithCredentials(connection map[string]interface{}, httpRequest *http.Request, headerValuePrefixes HeaderValuePrefixes, headerAlias HeaderAlias, timeout int32) (Result, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	result := Result{}
	log.Info(httpRequest.URL)
	if err := SetAuthenticationHeaders(connection, httpRequest, headerValuePrefixes, headerAlias); err != nil {
		log.Error(err)
		return result, err
	}

	if err := FixRequestURL(httpRequest); err != nil {
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


func (p *OpenApiPlugin) parseActionRequest(connection map[string]interface{}, executeActionRequest *plugin.ExecuteActionRequest) (*http.Request, error) {
	actionName := executeActionRequest.Name

	if !p.ActionExist(actionName) {
		err := errors.New("No such method")
		log.Error(err)
		return nil, err
	}

	actionName = mask.ReplaceActionAlias(actionName)
	operation := handlers.OperationDefinitions[actionName]

	// get the parameters from the request.
	rawParameters, err := executeActionRequest.GetParameters()

	if err != nil {
		return nil, err
	}

	// replace the raw parameters with their alias.
	requestParameters := mask.ReplaceActionParametersAliases(actionName, rawParameters)

	// add to request parameters
	paramsFromConnection := GetPathParamsFromConnection(connection, p.PathParams)

	for k, v := range paramsFromConnection {
		requestParameters[k] = v
	}

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

func GetPathParamsFromConnection(connection map[string]interface{}, params PathParams) map[string]string {
	paramsFromConnection := map[string]string{}

	for header, headerValue := range connection {
		if headerValueString, ok := headerValue.(string); ok {
			if stringInSlice(header, params) {
				paramsFromConnection[header] = headerValueString
			}
		}
	}

	return paramsFromConnection
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if strings.EqualFold(b, a) {
			return true
		}
	}
	return false
}

func GenerateMaskFile(c *cli.Context) error {
	apiPlugin, err := NewOpenApiPlugin(nil, PluginMetadata{
		Name:        "",
		MaskFile:    "",
		OpenApiFile: c.String("file"),
	}, PluginChecks{})

	if err != nil {
		return err
	}

	indexMap := map[string]int{}

	genAlias := func(str string) string {
		uppercaseWords := []string{"url", "id", "ip", "ssl"}

		// replace _ with ' '
		str = strings.ReplaceAll(str, "_", " ")

		// iter over words in the string
		for _, word := range strings.Split(str, " ") {

			// check if the word is in out list.
			if stringInSlice(word, uppercaseWords) {
				str = strings.ReplaceAll(str, word, strings.ToUpper(word))
			}
		}

		return strings.Join(strings.Fields(strings.Title(str)), " ")
	}

	err = RunTemplate(consts.MaskFile, consts.YAMLTemplate, apiPlugin, template.FuncMap{
		"actName": genAlias,
		"paramName": func(str string) string {
			a := strings.Split(genAlias(str),".")
			return a[len(a)-1]
		},
		"index": func(str string) int {
			if _, ok := indexMap[str]; ok {
				indexMap[str] += 1
				return indexMap[str]
			}
			indexMap[str] = 1
			return 1
		},
	})
	if err != nil {
		return err
	}

	return nil

}

func GenerateMarkdown(c *cli.Context) error {

	apiPlugin, err := NewOpenApiPlugin(nil, PluginMetadata{
		Name:        c.String("name"),
		MaskFile:    c.String("mask"),
		OpenApiFile: c.String("file"),
	}, PluginChecks{})

	if err != nil {
		return err
	}

	err = RunTemplate(consts.README, consts.READMETemplate, apiPlugin, nil)
	if err != nil {
		return err
	}

	return nil

}

func RunTemplate(fileName string, templateStr string, obj interface{}, funcs template.FuncMap) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	tmpl, err := template.New("").Funcs(funcs).Parse(templateStr)

	if err != nil {
		return err
	}

	if err := tmpl.Execute(f, obj); err != nil {
		return err
	}

	return nil
}

func NewOpenApiPlugin(connectionTypes map[string]connections.Connection, meta PluginMetadata, checks PluginChecks) (*OpenApiPlugin, error) {

	helpingFunctions = checks

	err := mask.ParseMask(meta.MaskFile)

	if err != nil {
		return nil, errors.Errorf("Cannot parse mask file: %s", meta.MaskFile)
	}

	description, actions, err := extractActions(meta.OpenApiFile)
	if err != nil {
		return nil, err
	}

	return &OpenApiPlugin{
		actions:             actions,
		HeaderValuePrefixes: meta.HeaderValuePrefixes,
		HeaderAlias:         meta.HeaderAlias,
		description: plugin.Description{
			Name:        meta.Name,
			Description: description,
			Tags:        meta.Tags,
			Connections: connectionTypes,
			Provider:    meta.Provider,
		},
	}, nil
}

func extractActions(OpenApiFile string) (string, []plugin.Action, error) {
	var actions []plugin.Action

	openApi, err := loadOpenApi(OpenApiFile)

	if err != nil {
		return "", nil, err
	}

	if len(openApi.Servers) == 0 {
		return "", nil, errors.New("no server URL provided in OpenApi file")
	}

	// Set default openApi server
	openApiServer := openApi.Servers[0]
	requestUrl = openApiServer.URL

	for urlVariableName, urlVariable := range openApiServer.Variables {
		requestUrl = strings.ReplaceAll(requestUrl, consts.ParamPrefix+urlVariableName+consts.ParamSuffix, urlVariable.Default)
	}

	err = handlers.DefineOperations(openApi)

	if err != nil {
		return "", nil, err
	}

	for _, operation := range handlers.OperationDefinitions {
		actionName := operation.OperationId

		// Skip masked actions
		if mask.MaskData.Actions != nil {
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
			paramDescription := pathParam.Schema.Description

			if paramDescription == "" {
				paramDescription = pathParam.Spec.Description
			}

			if actionParam := parseActionParam(action.Name, &paramName, pathParam.Spec.Schema, pathParam.Required, paramDescription); actionParam != nil {
				action.Parameters[paramName] = *actionParam
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

	// sort the actions
	// each time we parse the openapi the actions are added to the map in different order.
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name < actions[j].Name
	})

	return openApi.Info.Description, actions, nil
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
	handleBodyParamOfType(schema, parentPath, action)

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
			handleBodyParams(bodyProperty.Value, fullParamPath, action)
		} else {
			handleBodyParamOfType(bodyProperty.Value, fullParamPath, action)
			isParamRequired := false

			for _, requiredParam := range schema.Required {
				if propertyName == requiredParam {
					isParamRequired = true
					break
				}
			}

			if actionParam := parseActionParam(action.Name, &fullParamPath, bodyProperty, isParamRequired, bodyProperty.Value.Description); actionParam != nil {
				action.Parameters[fullParamPath] = *actionParam
			}
		}
	}
}

func handleBodyParamOfType(schema *openapi3.Schema, parentPath string, action *plugin.Action) {
	if schema.AllOf != nil || schema.AnyOf != nil || schema.OneOf != nil {

		allSchemas := []openapi3.SchemaRefs{schema.AllOf, schema.AnyOf, schema.OneOf}

		// find properties nested in Allof, Anyof, Oneof
		for _, schemaType := range allSchemas {
			for _, schemaParams := range schemaType {
				handleBodyParams(schemaParams.Value, parentPath, action)
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

func parseActionParam(actionName string, paramName *string, paramSchema *openapi3.SchemaRef, isParamRequired bool, paramDescription string) *plugin.ActionParameter {
	var isMulti bool
	paramType := paramSchema.Value.Type
	paramFormat := paramSchema.Value.Format

	paramOptions := getParamOptions(paramSchema.Value.Enum, &paramType)
	paramPlaceholder := getParamPlaceholder(paramSchema.Value.Example, paramType)
	paramDefault := getParamDefault(paramSchema.Value.Default, paramType)
	paramIndex := 999 // parameters will be ordered from lowest to highest in UI. This is the default, meaning - the end of the list.

	if mask.MaskData.Actions != nil {
		maskedParam := mask.MaskData.GetParameter(actionName, *paramName)
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
