package consts

const (
	TypeArray    = "array"
	TypeInteger  = "integer"
	TypeBoolean  = "boolean"
	TypeBool     = "bool"
	TypeObject   = "object"
	TypeJson     = "code:json"
	TypeDropdown = "dropdown"

	BodyParamDelimiter = "."
	RequestBodyType    = "application/json"
	URLEncoded         = "application/x-www-form-urlencoded"
	ParamPrefix        = "{"
	ParamSuffix        = "}"
	RequestUrlKey      = "REQUEST_URL"
	ArrayDelimiter     = ","
	ContentTypeHeader  = "Content-Type"

	BearerAuth        = "Bearer "
	BasicAuth         = "Basic "
	BasicAuthUsername = "USERNAME"
	BasicAuthPassword = "PASSWORD"

	ParamPlaceholderPrefix = "Example: "
	Error                  = 1
	OK                     = 0

	GzipFile               = ".gz"
	ENVStatusKey           = "PROD"
	ConnectionNotMandatory = "CONNECTION_IS_NOT_MANDATORY"
	TestConnectionFailed   = "Test Connection Failed"
)
