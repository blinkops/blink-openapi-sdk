package plugin

import (
	"encoding/json"
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
	"os"
	"sort"
	"strings"
	"time"
)

var (
	requestUrl string
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

func (p *openApiPlugin) MakeMarkdown() error {
	f, err := os.Create(consts.MarkdownFile)
	if err != nil {
		return err
	}

	defer f.Close()

	BlinkHeader := fmt.Sprintf("%s%s \n", consts.BlinkPrefix, p.Describe().Name)

	_, err = f.WriteString(BlinkHeader)
	if err != nil {
		return err
	}

	actions := p.GetActions()

	for _, action := range actions {

		// count the number of parameters.
		ParameterCount := len(action.Parameters)
		b := consts.Parameter
		// Parameters in plural
		if ParameterCount != 1 {
			b += "s"
		}

		// write the action name description and parameters count
		_, err = f.WriteString(fmt.Sprintf("## %s \n>%s ##### \n##### %d %s\n", action.Name, action.Description, ParameterCount, b))
		if err != nil {
			return err
		}

		isLongDescription := false
		var ParamBody strings.Builder

		// sort the parameters of the action.
		keys := make([]string, 0, len(action.Parameters))

		for k := range action.Parameters {
			keys = append(keys, k)

		}

		sort.Strings(keys)

		// write the action parameters and their description
		for _, key := range keys {

			param := action.Parameters[key]

			// long description mess up tables
			if len(param.Description) >= 100 {
				isLongDescription = true
				_, err = ParamBody.WriteString(fmt.Sprintf("\n#### %s\n* %s\n", key, param.Description))
				if err != nil {
					return err
				}
			} else {
				// add a table row
				_, err = ParamBody.WriteString(fmt.Sprintf("| %s | %s |</br>", key, param.Description))
				if err != nil {
					return err
				}
			}

		}

		// Don't show empty table for actions with no parameters or long description.
		if (ParameterCount == 1 && isLongDescription) || ParameterCount == 0 {

		} else {
			// write the table header
			_, err = f.WriteString(consts.TableHeader)
			if err != nil {
				return err
			}
		}

		// write the table to the file.
		_, err = f.WriteString(ParamBody.String())
		if err != nil {
			return err
		}

	}
	return nil
}

func (p *openApiPlugin) TestCredentials(map[string]connections.ConnectionInstance) (*plugin.CredentialsValidationResponse, error) {
	// ToDo: replace with real implementation
	return &plugin.CredentialsValidationResponse{
		AreCredentialsValid:   true,
		RawValidationResponse: nil,
	}, nil
}

func (p *openApiPlugin) ExecuteAction(actionContext *plugin.ActionContext, request *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error) {
	client := &http.Client{
		Timeout: time.Duration(request.Timeout) * time.Second,
	}
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

	requestUrl = p.getRequestUrl(actionContext)

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

func NewOpenApiPlugin(name string, provider string, tags []string, connectionTypes map[string]connections.Connection, openApiFile string, maskFile string) (*openApiPlugin, error) {
	var actions []plugin.Action

	openApi, err := loadOpenApi(openApiFile)

	if err != nil {
		return nil, err
	}

	err = mask.ParseMask(maskFile)

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

	// sort the actions
	// each time we parse the openapi the actions are added to the map in different order.
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name < actions[j].Name
	})

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

func buildResponse(response *http.Response) ([]byte, error) {
	defer func() {
		_ = response.Body.Close()
	}()

	var jsonMap map[string]interface{}

	result, err := ioutil.ReadAll(response.Body)

	// unmarshal to check that the json body is valid.
	err = json.Unmarshal(result, &jsonMap)
	if err != nil {
		return nil, err
	}

	parsedOutput, err := json.Marshal(jsonMap)
	if err != nil {
		return nil, err
	}

	return parsedOutput, nil
}
