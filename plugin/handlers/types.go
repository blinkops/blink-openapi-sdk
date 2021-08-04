package handlers

import "github.com/getkin/kin-openapi/openapi3"

var (
	OperationDefinitions = map[string]*OperationDefinition{}
)

// OperationDefinition This structure describes an Operation
type OperationDefinition struct {
	OperationId         string                // The operation_id description from OpenApi, used to generate function names
	PathParams          []parameterDefinition // Parameters in the path, eg, /path/:param
	HeaderParams        []parameterDefinition // Parameters in HTTP headers
	QueryParams         []parameterDefinition // Parameters in the query, /path?param
	CookieParams        []parameterDefinition // Parameters in cookies
	TypeDefinitions     []typeDefinition      // These are all the types we need to define for this operation
	SecurityDefinitions []securityDefinition  // These are the security providers
	BodyRequired        bool
	Bodies              []RequestBodyDefinition // The list of Bodies for which to generate handlers.
	Summary             string                  // Summary string from OpenApi, used to generate a comment
	Method              string                  // GET, POST, DELETE, etc.
	Path                string                  // The OpenApi path for the operation, like /resource/{id}
	Spec                *openapi3.Operation
}

// params Returns the list of all parameters except path parameters. path parameters
// are handled differently from the rest, since they're mandatory.
func (o *OperationDefinition) params() []parameterDefinition {
	result := append(o.QueryParams, o.HeaderParams...)
	result = append(result, o.CookieParams...)
	return result
}

// AllParams Returns all parameters
func (o *OperationDefinition) AllParams() []parameterDefinition {
	result := append(o.QueryParams, o.HeaderParams...)
	result = append(result, o.CookieParams...)
	result = append(result, o.PathParams...)
	return result
}



func (o OperationDefinition) GetDefaultBody() *RequestBodyDefinition {
	for _, paramBody := range o.Bodies {
		if paramBody.DefaultBody {
			return &paramBody
		}
	}

	return nil
}

func (o OperationDefinition) GetDefaultBodyType() string {
	defaultBody := o.GetDefaultBody()

	if o.GetDefaultBody() != nil {
		return defaultBody.ContentType
	}

	return ""
}

// parameterDefinition describes the various request parameters
type parameterDefinition struct {
	ParamName string // The original json parameter name, eg param_name
	In        string // Where the parameter is defined - path, header, cookie, query
	Required  bool   // Is this a required parameter?
	Spec      *openapi3.Parameter
	Schema    schema
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
	GoType                   string            // The Go type needed to represent the schema
	RefType                  string            // If the type has a type name, this is set
	ArrayType                *schema           // The schema of array element
	EnumValues               map[string]string // Enum values
	Properties               []property        // For an object, the fields with names
	HasAdditionalProperties  bool              // Whether we support additional properties
	AdditionalPropertiesType *schema           // And if we do, their type
	AdditionalTypes          []typeDefinition  // We may need to generate auxiliary helper types, stored here
	SkipOptionalPointer      bool              // Some types don't need a * in front when they're optional
	Description              string            // The description of the element
	OApiSchema               *openapi3.Schema  // The original OpenAPIv3 schema.
}

// securityDefinition describes required authentication headers
type securityDefinition struct {
	providerName string
	scopes       []string
}

// RequestBodyDefinition This describes a request body
type RequestBodyDefinition struct {
	// Is this body required, or optional?
	Required bool

	// This is the schema describing this body
	Schema schema

	// When we generate type names, we need a Tag for it, such as JSON, in
	// which case we will produce "JSONBody".
	NameTag string

	// This is the content type corresponding to the body, eg, application/json
	ContentType string

	// Whether this is the default body type. For an operation named OpFoo, we
	// will not add suffixes like OpFooJSONBody for this one.
	DefaultBody bool
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
		if param.ParamName == name {
			return &param
		}
	}
	return nil
}
