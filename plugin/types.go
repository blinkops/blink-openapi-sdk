package plugin

import "github.com/getkin/kin-openapi/openapi3"

// operationDefinition This structure describes an Operation
type operationDefinition struct {
	operationId         string                // The operation_id description from OpenApi, used to generate function names
	pathParams          []parameterDefinition // Parameters in the path, eg, /path/:param
	headerParams        []parameterDefinition // Parameters in HTTP headers
	queryParams         []parameterDefinition // Parameters in the query, /path?param
	cookieParams        []parameterDefinition // Parameters in cookies
	typeDefinitions     []typeDefinition      // These are all the types we need to define for this operation
	securityDefinitions []securityDefinition  // These are the security providers
	bodyRequired        bool
	bodies              []requestBodyDefinition // The list of bodies for which to generate handlers.
	summary             string                  // summary string from OpenApi, used to generate a comment
	method              string                  // GET, POST, DELETE, etc.
	path                string                  // The OpenApi path for the operation, like /resource/{id}
	spec                *openapi3.Operation
}

// params Returns the list of all parameters except path parameters. path parameters
// are handled differently from the rest, since they're mandatory.
func (o *operationDefinition) params() []parameterDefinition {
	result := append(o.queryParams, o.headerParams...)
	result = append(result, o.cookieParams...)
	return result
}

// allParams Returns all parameters
func (o *operationDefinition) allParams() []parameterDefinition {
	result := append(o.queryParams, o.headerParams...)
	result = append(result, o.cookieParams...)
	result = append(result, o.pathParams...)
	return result
}

// parameterDefinition describes the various request parameters
type parameterDefinition struct {
	paramName string // The original json parameter name, eg param_name
	in        string // Where the parameter is defined - path, header, cookie, query
	required  bool   // Is this a required parameter?
	spec      *openapi3.Parameter
	schema    schema
}

// typeDefinition describes a Go type definition in generated code.
//
// Let's use this example schema:
// components:
//  schemas:
//    Person:
//      type: object
//      properties:
//      name:
//        type: string
type typeDefinition struct {
	// The name of the type, eg, type <...> Person
	typeName string

	// The name of the corresponding JSON description, as it will sometimes
	// differ due to invalid characters.
	jsonName string

	// This is the schema wrapper is used to populate the type description
	schema schema
}

// schema This describes a schema, a type definition.
type schema struct {
	goType                   string            // The Go type needed to represent the schema
	refType                  string            // If the type has a type name, this is set
	arrayType                *schema           // The schema of array element
	enumValues               map[string]string // Enum values
	properties               []property        // For an object, the fields with names
	hasAdditionalProperties  bool              // Whether we support additional properties
	additionalPropertiesType *schema           // And if we do, their type
	additionalTypes          []typeDefinition  // We may need to generate auxiliary helper types, stored here
	skipOptionalPointer      bool              // Some types don't need a * in front when they're optional
	description              string            // The description of the element
	oApiSchema               *openapi3.Schema  // The original OpenAPIv3 schema.
}

// securityDefinition describes required authentication headers
type securityDefinition struct {
	providerName string
	scopes       []string
}

// requestBodyDefinition This describes a request body
type requestBodyDefinition struct {
	// Is this body required, or optional?
	required bool

	// This is the schema describing this body
	schema schema

	// When we generate type names, we need a Tag for it, such as JSON, in
	// which case we will produce "JSONBody".
	nameTag string

	// This is the content type corresponding to the body, eg, application/json
	contentType string

	// Whether this is the default body type. For an operation named OpFoo, we
	// will not add suffixes like OpFooJSONBody for this one.
	defaultBody bool
}

// property describes a request body key
type property struct {
	description    string
	jsonFieldName  string
	schema         schema
	required       bool
	nullable       bool
	extensionProps *openapi3.ExtensionProps
}

type parameterDefinitions []parameterDefinition

func (p parameterDefinitions) FindByName(name string) *parameterDefinition {
	for _, param := range p {
		if param.paramName == name {
			return &param
		}
	}
	return nil
}
