package consts

const (
	TypeArray    = "array"
	TypeInteger  = "integer"
	TypeBoolean  = "boolean"
	TypeObject   = "object"
	TypeDropdown = "dropdown"

	BodyParamDelimiter = "."
	RequestBodyType    = "application/json"
	URLEncoded         = "application/x-www-form-urlencoded"
	ParamPrefix        = "{"
	ParamSuffix        = "}"
	RequestUrlKey      = "REQUEST_URL"
	ArrayDelimiter     = ","
	ContentTypeHeader  = "Content-Type"

	ParamPlaceholderPrefix = "Example: "

	READMETemplate = "## blink-{{ .Describe.Name }}\n\n> {{ .Describe.Description }}\n\n{{range .GetActions}}\n## {{.Name }}\n* {{.Description }}\n<table>\n<caption>Action Parameters</caption>\n  <thead>\n    <tr>\n        <th>Param Name</th>\n        <th>Param Description</th>\n    </tr>\n  </thead>\n  <tbody>\n    <tr>{{ range $name, $param := .Parameters}}\n       <tr>\n           <td>{{ $name }}</td>\n           <td>{{ $param.Description }}</td>\n       </tr>{{ end}}\n    </tr>\n  </tbody>\n</table>\n{{ end}}"

	README = "README.md"

	Error = 1
	OK    = 0
)
