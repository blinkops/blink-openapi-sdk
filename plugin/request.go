package plugin

import (
	"encoding/json"
	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/getkin/kin-openapi/openapi3"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"strings"
)

func parseCookieParams(requestParameters map[string]string, operation *handlers.OperationDefinition, request *http.Request) {
	for paramName, paramValue := range requestParameters {
		for _, cookieParam := range operation.CookieParams {
			if paramName == cookieParam.ParamName {
				cookie := &http.Cookie{
					Name:  paramName,
					Value: paramValue,
				}

				request.AddCookie(cookie)
			}
		}
	}
}

func parseHeaderParams(requestParameters map[string]string, operation *handlers.OperationDefinition, request *http.Request) {
	for paramName, paramValue := range requestParameters {
		for _, headerParam := range operation.HeaderParams {
			if paramName == headerParam.ParamName {
				request.Header.Set(paramName, paramValue)
			}
		}
	}
}

func parsePathParams(requestParameters map[string]string, operation *handlers.OperationDefinition, path string) string {
	requestPath := path

	for paramName, paramValue := range requestParameters {
		for _, pathParam := range operation.PathParams {
			if paramName == pathParam.ParamName {
				requestPath = strings.ReplaceAll(path, consts.ParamPrefix+paramName+consts.ParamSuffix, paramValue)
			}
		}
	}

	return requestPath
}

func parseQueryParams(requestParameters map[string]string, operation *handlers.OperationDefinition, request *http.Request) {

	query := request.URL.Query()
	for paramName, paramValue := range requestParameters {

		for _, queryParam := range operation.QueryParams {
			if paramName == queryParam.ParamName {
				query.Add(paramName, paramValue)
			}

		}
	}

	request.URL.RawQuery = query.Encode()


}


func parseBodyParams(requestParameters map[string]string, operation *handlers.OperationDefinition) ([]byte, error) {
	requestBody := map[string]interface{}{}
	operationBody := &handlers.RequestBodyDefinition{}

	// Looking for a json type body schema
	for _, paramBody := range operation.Bodies {
		if strings.ToLower(paramBody.ContentType) == consts.RequestBodyType {
			operationBody = &paramBody
			break
		}
	}

	// Add "." delimited params as request body
	for paramName, paramValue := range requestParameters {
		if strings.Contains(paramName, consts.BodyParamDelimiter) {
			mapKeys := strings.Split(paramName, consts.BodyParamDelimiter)
			buildRequestBody(mapKeys, operationBody.Schema.OApiSchema, paramValue, requestBody)
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
		subPropertySchema := handlers.GetPropertyByName(key, propertySchema)

		if subPropertySchema != nil {
			requestBody[mapKeys[len(mapKeys)-1]] = castBodyParamType(paramValue, subPropertySchema.Type)
		} else {
			log.Errorf("Invalid request body param passed: %s", key)
		}

	} else {
		if _, ok := requestBody[key]; !ok {
			requestBody[key] = map[string]interface{}{}
		}

		subPropertySchema := handlers.GetPropertyByName(key, propertySchema)
		buildRequestBody(mapKeys[1:], subPropertySchema, paramValue, requestBody[key].(map[string]interface{}))
	}
}

// Cast proper parameter types when building json request body
func castBodyParamType(paramValue string, paramType string) interface{} {
	switch paramType {
	case consts.TypeInteger:
		if intValue, err := strconv.Atoi(paramValue); err != nil {
			return paramValue
		} else {
			return intValue
		}
	case consts.TypeBoolean:
		if boolValue, err := strconv.ParseBool(paramValue); err != nil {
			return paramValue
		} else {
			return boolValue
		}
	case consts.TypeArray:
		return strings.Split(paramValue, consts.ArrayDelimiter)
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

	if explicitRequestUrl, ok := connection[consts.RequestUrlKey].(string); ok {
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
	delete(connection, consts.RequestUrlKey)

	return connection, nil
}
