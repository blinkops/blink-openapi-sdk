package plugin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
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
			if strings.EqualFold(paramName, pathParam.ParamName) {
				requestPath = strings.ReplaceAll(requestPath, consts.ParamPrefix+pathParam.ParamName+consts.ParamSuffix, url.QueryEscape(paramValue))
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
		request.ContentLength = int64(len(marshaledBody))
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
	case consts.TypeObject:
		if paramValue == "" {
			paramValue = "{}"
		}

		var jsonValue map[string]interface{}
		if err := json.Unmarshal([]byte(paramValue), &jsonValue); err != nil {
			return paramValue
		}

		return jsonValue
	default:
		return paramValue
	}
}

// SetAuthenticationHeaders Credentials should be saved as headerName -> value according to the api definition
func setAuthenticationHeaders(securityHeaders map[string]interface{}, request *http.Request, getTokenFromCredentials GetTokenFromCredentials, prefixes HeaderValuePrefixes, headerAlias HeaderAlias) error {

	// If a GetTokenFromCredentials was passed AND there's no "Token" key, generate a token with the function.
	// "Token" is prioritized to allow OAuth
	if _, ok := securityHeaders["Token"]; getTokenFromCredentials != nil && !ok {
		return setCustomHeader(securityHeaders, request, getTokenFromCredentials, prefixes)
	}

	headers := make(map[string]string)
	for header, headerValue := range securityHeaders {
		if headerValueString, ok := headerValue.(string); ok {
			header = strings.ToUpper(header)
			prefix := getPrefix(header, headerValueString, prefixes)
			if newHeader, ok := headerAlias[header]; ok {
				header = strings.ToUpper(newHeader)
				if prefix == "" {
					prefix = getPrefix(header, headerValueString, prefixes)
				}
			}
			headerValueString = prefix + headerValueString

			// If the user supplied BOTH username and password
			// Username:Password pair should be base64 encoded
			// and sent as "Authorization: base64(user:pass)"
			headers[header] = headerValueString
			if username, ok := headers[consts.BasicAuthUsername]; ok {
				if password, ok := headers[consts.BasicAuthPassword]; ok {
					header, headerValueString = "Authorization", constructBasicAuthHeader(username, password)
					cleanRedundantHeaders(&request.Header)
				}
			}

			request.Header.Set(strings.ToUpper(header), headerValueString)
		}
	}
	return nil
}

// we want to help the user by adding prefixes he might have missed
// example:   Bearer <TOKEN>
func getPrefix(header string, value string, prefixes HeaderValuePrefixes) string {
	if val, ok := prefixes[header]; ok {
		if !strings.HasPrefix(value, val) {
			return val
		}
	}
	return ""
}

func constructBasicAuthHeader(username, password string) string {
	data := []byte(fmt.Sprintf("%s:%s", username, password))
	hashed := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("%s%s", consts.BasicAuth, hashed)
}

func cleanRedundantHeaders(requestHeaders *http.Header) {
	requestHeaders.Del(consts.BasicAuthUsername)
	requestHeaders.Del(consts.BasicAuthPassword)
}

func setCustomHeader(securityHeaders map[string]interface{}, request *http.Request, getTokenFromCredentials GetTokenFromCredentials, prefixes HeaderValuePrefixes) error {
	generatedToken, err := getTokenFromCredentials(securityHeaders)
	if err != nil {
		return err
	}
	if generatedToken != "" {
		for headerKey, headerPrefix := range prefixes {
			request.Header.Set(headerKey, headerPrefix+generatedToken)
			return nil
		}
	}
	log.Info("In order to generate a token with a getTokenFromCredentials function, there has to be one 'prefixes' pair")
	return errors.New("No prefixes found to be paired with the token")
}

func getRequestUrlFromConnection(requestUrl string, connection map[string]interface{}) string {
	if explicitRequestUrl, ok := connection[consts.RequestUrlKey].(string); ok {
		requestUrl = explicitRequestUrl
	}
	return requestUrl
}

func getCredentials(actionContext *plugin.ActionContext, provider string) (map[string]interface{}, error) {
	connection, err := actionContext.GetCredentials(provider)
	if err != nil {
		return nil, err
	}
	return connection, nil
}
