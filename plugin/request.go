package plugin

import (
	"encoding/json"
	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/getkin/kin-openapi/openapi3"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// parseCookieParams puts the cookie params in the cookie part of the request.
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

// parseHeaderParams puts the header params in the header of the request.
func parseHeaderParams(requestParameters map[string]string, operation *handlers.OperationDefinition, request *http.Request) {
	for paramName, paramValue := range requestParameters {
		for _, headerParam := range operation.HeaderParams {
			if paramName == headerParam.ParamName {
				request.Header.Set(paramName, paramValue)
			}
		}
	}

}

// parsePathParams puts the path params path of the request.
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

// parseQueryParams adds the query params as urlencoded to the request.
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

// parseBodyParams add the params to to body of the request (JSON/ URL encoded params).
func parseBodyParams(requestParameters map[string]string, operation *handlers.OperationDefinition, request *http.Request) error {
	requestBody := map[string]interface{}{}

	// the default body prefers to be json if available, otherwise will pick the first body.
	defaultBody := operation.GetDefaultBody()

	// some request do not have body like GET.
	if defaultBody == nil {
		return nil
	}

	// Add "." delimited params as request body
	for paramName, paramValue := range requestParameters {
		mapKeys := strings.Split(paramName, consts.BodyParamDelimiter)
		buildRequestBody(mapKeys, defaultBody.Schema.OApiSchema, paramValue, requestBody)

	}

	// when the content type is url encoded, the values need be urlencoded and sent in the body.
	if defaultBody.ContentType == consts.URLEncoded {
		values := url.Values{}
		// add the values
		for paramName, paramValue := range requestBody {
			values.Add(paramName, paramValue.(string))
		}

		//url encoded the values and add to the body.
		request.Body = ioutil.NopCloser(strings.NewReader(values.Encode()))

	} else {
		// for any other content type, send the values as JSON.
		marshaledBody, err := json.Marshal(requestBody)

		if err != nil {
			return err
		}

		// add the JSON to the body.
		request.Body = ioutil.NopCloser(strings.NewReader(string(marshaledBody)))
	}
	return nil
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
func SetAuthenticationHeaders(actionContext *plugin.ActionContext, request *http.Request, provider string, HeaderPrefixes HeaderPrefixes) error {
	securityHeaders, err := GetCredentials(actionContext, provider)

	if err != nil {
		return err
	}

	for header, headerValue := range securityHeaders {
		if headerValueString, ok := headerValue.(string); ok {
			header = strings.ToUpper(header)
			if val, ok := HeaderPrefixes[header]; ok {

				// we want to help the user by adding prefixes he might have missed
				// for example
				// Bearer <TOKEN>
				if !strings.HasPrefix(headerValueString, val) { // check what prefix the user doesn't have
					// add the prefix
					headerValueString = val + headerValueString
				}

			}

			request.Header.Set(header, headerValueString)
		}
	}

	return nil
}

func GetRequestUrl(actionContext *plugin.ActionContext, provider string) string {
	connection, err := actionContext.GetCredentials(provider)

	if err != nil {
		return requestUrl
	}

	if explicitRequestUrl, ok := connection[consts.RequestUrlKey].(string); ok {
		return explicitRequestUrl
	}

	return requestUrl
}

func GetCredentials(actionContext *plugin.ActionContext, provider string) (jsonMap, error) {
	connection, err := actionContext.GetCredentials(provider)

	if err != nil {
		return nil, err
	}

	// Remove request url and leave only other authentication headers
	delete(connection, consts.RequestUrlKey)

	return connection, nil
}
