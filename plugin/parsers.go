package plugin

import (
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	"regexp"
	"sort"
	"strings"
)

var (
	pathParamRE = regexp.MustCompile("{[.;?]?([^{}*]+)\\*?}")
)

// defineOperations returns all operations for an openApi definition.
func defineOperations(openApi *openapi3.T) error {
	for _, requestPath := range sortedPathsKeys(openApi.Paths) {
		pathItem := openApi.Paths[requestPath]
		// These are parameters defined for all methods on a given path. They
		// are shared by all methods.
		globalParams, err := describeParameters(pathItem.Parameters)
		if err != nil {
			return fmt.Errorf("error describing global parameters for %s: %s", requestPath, err)
		}

		// Each path can have a number of operations, POST, GET, OPTIONS, etc.
		pathOps := pathItem.Operations()
		for _, opName := range sortedOperationsKeys(pathOps) {
			op := pathOps[opName]
			if pathItem.Servers != nil {
				op.Servers = &pathItem.Servers
			}

			// These are parameters defined for the specific path method that
			// we're iterating over.
			localParams, err := describeParameters(op.Parameters)
			if err != nil {
				return fmt.Errorf("error describing global parameters for %s/%s: %s", opName, requestPath, err)
			}
			// All the parameters required by a handler are the union of the
			// global parameters and the local parameters.
			allParams := append(globalParams, localParams...)

			// Order the path parameters to match the order as specified in
			// the path, not in the openApi spec, and validate that the parameter
			// names match, as downstream code depends on that.
			pathParams := filterParameterDefinitionByType(allParams, "path")
			pathParams, err = sortParamsByPath(requestPath, pathParams)
			if err != nil {
				return err
			}

			bodyDefinitions, typeDefinitions, err := generateBodyDefinitions(op.OperationID, op.RequestBody)
			if err != nil {
				return errors.Wrap(err, "error generating body definitions")
			}

			opDef := operationDefinition{
				pathParams:   pathParams,
				headerParams: filterParameterDefinitionByType(allParams, "header"),
				queryParams:  filterParameterDefinitionByType(allParams, "query"),
				cookieParams: filterParameterDefinitionByType(allParams, "cookie"),
				operationId:  op.OperationID,
				// Replace newlines in summary.
				summary:         op.Summary,
				method:          opName,
				path:            requestPath,
				spec:            op,
				bodies:          bodyDefinitions,
				typeDefinitions: typeDefinitions,
			}

			// check for overrides of securityDefinitions.
			// See: "Step 2. Applying security:" from the spec:
			// https://swagger.io/docs/specification/authentication/
			if op.Security != nil {
				opDef.securityDefinitions = describeSecurityDefinition(*op.Security)
			} else {
				// use global securityDefinitions
				// globalSecurityDefinitions contains the top-level securityDefinitions.
				// They are the default securityPermissions which are injected into each
				// path, except for the case where a path explicitly overrides them.
				opDef.securityDefinitions = describeSecurityDefinition(openApi.Security)

			}

			if op.RequestBody != nil {
				opDef.bodyRequired = op.RequestBody.Value.Required
			}

			// Generate all the type definitions needed for this operation
			opDef.typeDefinitions = append(opDef.typeDefinitions, generateTypeDefsForOperation(opDef)...)

			operationDefinitions[opDef.operationId] = &opDef
		}
	}

	return nil
}

// sortedPathsKeys This function is the same as above, except it sorts the keys for a Paths
// dictionary.
func sortedPathsKeys(dict openapi3.Paths) []string {
	keys := make([]string, len(dict))
	i := 0
	for key := range dict {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return keys
}

// describeParameters This function walks the given parameters dictionary, and generates the above
// descriptors into a flat list. This makes it a lot easier to traverse the
// data in the template engine.
func describeParameters(params openapi3.Parameters) ([]parameterDefinition, error) {
	outParams := make([]parameterDefinition, 0)
	for _, paramOrRef := range params {
		param := paramOrRef.Value

		pd := parameterDefinition{
			paramName: param.Name,
			in:        param.In,
			required:  param.Required,
			spec:      param,
		}
		outParams = append(outParams, pd)
	}
	return outParams, nil
}

// sortedOperationsKeys This function returns Operation dictionary keys in sorted order
func sortedOperationsKeys(dict map[string]*openapi3.Operation) []string {
	keys := make([]string, len(dict))
	i := 0
	for key := range dict {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return keys
}

// filterParameterDefinitionByType This function returns the subset of the specified parameters which are of the
// specified type.
func filterParameterDefinitionByType(params []parameterDefinition, in string) []parameterDefinition {
	var out []parameterDefinition
	for _, p := range params {
		if p.in == in {
			out = append(out, p)
		}
	}
	return out
}

// sortParamsByPath Reorders the given parameter definitions to match those in the path URI.
func sortParamsByPath(path string, in []parameterDefinition) ([]parameterDefinition, error) {
	pathParams := orderedParamsFromUri(path)
	n := len(in)
	if len(pathParams) != n {
		return nil, fmt.Errorf("path '%s' has %d positional parameters, but spec has %d declared",
			path, len(pathParams), n)
	}
	out := make([]parameterDefinition, len(in))
	for i, name := range pathParams {
		p := parameterDefinitions(in).FindByName(name)
		if p == nil {
			return nil, fmt.Errorf("path '%s' refers to parameter '%s', which doesn't exist in specification",
				path, name)
		}
		out[i] = *p
	}
	return out, nil
}

// orderedParamsFromUri Returns the argument names, in order, in a given URI string, so for
// /path/{param1}/{.param2*}/{?param3}, it would return param1, param2, param3
func orderedParamsFromUri(uri string) []string {
	matches := pathParamRE.FindAllStringSubmatch(uri, -1)
	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m[1]
	}
	return result
}

// describeSecurityDefinition describes request authentication requirements
func describeSecurityDefinition(securityRequirements openapi3.SecurityRequirements) []securityDefinition {
	outDefs := make([]securityDefinition, 0)

	for _, sr := range securityRequirements {
		for _, k := range sortedSecurityRequirementKeys(sr) {
			v := sr[k]
			outDefs = append(outDefs, securityDefinition{providerName: k, scopes: v})
		}
	}

	return outDefs
}

// sortedSecurityRequirementKeys sort security requirement keys
func sortedSecurityRequirementKeys(sr openapi3.SecurityRequirement) []string {
	keys := make([]string, len(sr))
	i := 0
	for key := range sr {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return keys
}

// generateBodyDefinitions This function turns the OpenApi body definitions into a list of our body
// definitions which will be used for code generation.
func generateBodyDefinitions(operationID string, bodyOrRef *openapi3.RequestBodyRef) ([]requestBodyDefinition, []typeDefinition, error) {
	if bodyOrRef == nil {
		return nil, nil, nil
	}
	body := bodyOrRef.Value

	var bodyDefinitions []requestBodyDefinition
	var typeDefinitions []typeDefinition

	for contentType, content := range body.Content {
		var tag string
		var defaultBody bool

		switch contentType {
		case requestBodyType:
			tag = "JSON"
			defaultBody = true
		default:
			continue
		}

		bodyTypeName := operationID + tag + "Body"
		bodySchema, err := generateGoSchema(content.Schema)
		if err != nil {
			return nil, nil, errors.Wrap(err, "error generating request body definition")
		}

		// If the request has a body, but it's not a user defined
		// type under #/components, we'll define a type for it, so
		// that we have an easy to use type for marshaling.
		if bodySchema.refType == "" {
			td := typeDefinition{
				typeName: bodyTypeName,
				schema:   bodySchema,
			}
			typeDefinitions = append(typeDefinitions, td)
			// The body schema now is a reference to a type
			bodySchema.refType = bodyTypeName
		}

		bd := requestBodyDefinition{
			required:    body.Required,
			schema:      bodySchema,
			nameTag:     tag,
			contentType: contentType,
			defaultBody: defaultBody,
		}
		bodyDefinitions = append(bodyDefinitions, bd)
	}
	return bodyDefinitions, typeDefinitions, nil
}

// generateGoSchema generate the OpenApi schema
func generateGoSchema(sref *openapi3.SchemaRef) (schema, error) {
	// Add a fallback value in case the sref is nil.
	// i.e. the parent schema defines a type:array, but the array has
	// no items defined. Therefore we have at least valid Go-Code.
	if sref == nil {
		return schema{goType: "interface{}"}, nil
	}

	oApiSchema := sref.Value
	return schema{oApiSchema: oApiSchema}, nil
}

func generateTypeDefsForOperation(op operationDefinition) []typeDefinition {
	var typeDefs []typeDefinition
	// Start with the params object itself
	if len(op.params()) != 0 {
		typeDefs = append(typeDefs, generateParamsTypes(op)...)
	}

	// Now, go through all the additional types we need to declare.
	for _, param := range op.allParams() {
		typeDefs = append(typeDefs, param.schema.getAdditionalTypeDefs()...)
	}

	for _, body := range op.bodies {
		typeDefs = append(typeDefs, body.schema.getAdditionalTypeDefs()...)
	}
	return typeDefs
}

func (s schema) getAdditionalTypeDefs() []typeDefinition {
	var result []typeDefinition
	for _, p := range s.properties {
		result = append(result, p.schema.getAdditionalTypeDefs()...)
	}
	result = append(result, s.additionalTypes...)
	return result
}

// generateParamsTypes This defines the schema for a parameters definition object which encapsulates
// all the query, header and cookie parameters for an operation.
func generateParamsTypes(op operationDefinition) []typeDefinition {
	var typeDefs []typeDefinition

	objectParams := op.queryParams
	objectParams = append(objectParams, op.headerParams...)
	objectParams = append(objectParams, op.cookieParams...)

	typeName := op.operationId + "params"

	s := schema{}
	for _, param := range objectParams {
		pSchema := param.schema
		if pSchema.hasAdditionalProperties {
			propRefName := strings.Join([]string{typeName, param.paramName}, "_")
			pSchema.refType = propRefName
			typeDefs = append(typeDefs, typeDefinition{
				typeName: propRefName,
				schema:   param.schema,
			})
		}
		prop := property{
			description:    param.spec.Description,
			jsonFieldName:  param.paramName,
			required:       param.required,
			schema:         pSchema,
			extensionProps: &param.spec.ExtensionProps,
		}
		s.properties = append(s.properties, prop)
	}

	s.description = op.spec.Description
	td := typeDefinition{
		typeName: typeName,
		schema:   s,
	}
	return append(typeDefs, td)
}

func getPropertyByName(name string, propertySchema *openapi3.Schema) *openapi3.Schema {
	var subPropertySchema *openapi3.Schema

	for propertyName, bodyProperty := range propertySchema.Properties {
		if propertyName == name {
			subPropertySchema = bodyProperty.Value
			break
		}
	}

	return subPropertySchema
}
