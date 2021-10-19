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

	READMETemplate = `
## blink-{{ .Describe.Name }}
> {{ .Describe.Description }}
{{range .GetActions}}
## {{.Name }}
* {{.Description }}
<table>
<caption>Action Parameters</caption>
  <thead>
    <tr>
        <th>Param Name</th>
        <th>Param Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>{{ range $name, $param := .Parameters}}
       <tr>
           <td>{{ $name }}</td>
           <td>{{ $param.Description }}</td>
       </tr>{{ end}}
    </tr>
  </tbody>
</table>
{{ end}}`

	README = "README.md"

	Error = 1
	OK    = 0

	YAMLTemplate = `
actions:{{range $action := .GetActions}}
  {{$action.Name }}:
    alias: {{ actName $action.Name }}
    parameters:{{ range $name, $param := .Parameters}}
      {{ $name }}:
        alias: "{{ paramName $name }}"
        index: {{ index $action.Name }}{{ end}}
{{ end}}`

	MaskFile = "mask.yaml"
)
