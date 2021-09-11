package handlers

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"

	"testing"
)

var paramDefinitionTestData []parameterDefinition

var testData = `[{
  "ParamName": "userId",
  "In": "path",
  "Required": true,
  "Spec": {
    "description": "user ID to add to somewhere",
    "in": "path",
    "name": "userId",
    "required": true,
    "schema": {
      "format": "int64",
      "type": "integer"
    }
  },
  "Schema": {
    "GoType": "",
    "RefType": "",
    "ArrayType": null,
    "EnumValues": null,
    "Properties": null,
    "HasAdditionalProperties": false,
    "AdditionalPropertiesType": null,
    "AdditionalTypes": null,
    "SkipOptionalPointer": false,
    "Description": "",
    "OApiSchema": null
  }
},
{
  "ParamName": "teamId",
  "In": "path",
  "Required": true,
  "Spec": {
    "description": "Team ID to add member to",
    "in": "path",
    "name": "teamId",
    "required": true,
    "schema": {
      "format": "int64",
      "type": "integer"
    }
  },
  "Schema": {
    "GoType": "",
    "RefType": "",
    "ArrayType": null,
    "EnumValues": null,
    "Properties": null,
    "HasAdditionalProperties": false,
    "AdditionalPropertiesType": null,
    "AdditionalTypes": null,
    "SkipOptionalPointer": false,
    "Description": "",
    "OApiSchema": null
  }
}]
`

// Since the data is complex and hard to generate (tried a few generators but
// they won't generate data for interface{} types which we have)
// so I created a test event data and we'll be using it for our tests
func TestMain(t *testing.T) {
	rawIn := json.RawMessage(testData)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(bytes, &paramDefinitionTestData)
	if err != nil {
		panic(err)
	}
}

func TestSortedPathKeys(t *testing.T) {
	type args struct {
		pathKeys openapi3.Paths
	}

	tests := []struct {
		name   string
		args   args
		output []string
	}{
		{
			name: "unsorted path keys",
			args: args{pathKeys: openapi3.Paths{
				"/api/dashboards/db":   nil,
				"/api/wallak/aaa":      nil,
				"/api/test/bbb":        nil,
				"/api/api/api/api/bbb": nil,
			},
			},
			output: []string{"/api/api/api/api/bbb", "/api/dashboards/db", "/api/test/bbb", "/api/wallak/aaa"},
		},
		{
			name: "already sorted path keys",
			args: args{pathKeys: openapi3.Paths{
				"/api/api/api/api/bbb": nil,
				"/api/dashboards/db":   nil,
				"/api/test/bbb":        nil,
				"/api/wallak/aaa":      nil,
			},
			},
			output: []string{"/api/api/api/api/bbb", "/api/dashboards/db", "/api/test/bbb", "/api/wallak/aaa"},
		},
	}
	for _, tt := range tests {
		t.Run("test sortedPathKeys(): "+tt.name, func(t *testing.T) {
			mySlice := sortedPathsKeys(tt.args.pathKeys)
			for i := range mySlice {
				if mySlice[i] != tt.output[i] {
					t.Errorf("sortedPathKeys() test failed: %v is not identical to %v", mySlice, tt.output)
				}
			}
		})
	}
}

func TestDescribeParameters(t *testing.T) {
	type args struct {
		params openapi3.Parameters
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "Parameters to describe",
			args: args{params: openapi3.Parameters{
				&openapi3.ParameterRef{
					Ref: "wallak",
					Value: &openapi3.Parameter{
						Name:        "teamId",
						In:          "path",
						Description: "some-desc",
						Required:    true,
						Schema: &openapi3.SchemaRef{
							Ref: "wa",
							Value: &openapi3.Schema{
								Format: "int64",
								Type:   "integer",
							},
						},
					},
				},
			},
			},
		},
	}

	for _, tt := range tests {
		t.Run("test describeParameters(): "+tt.name, func(t *testing.T) {
			result := describeParameters(tt.args.params)
			assert.Equal(t, len(result), 1)
			assert.Equal(t, result[0].ParamName, tt.args.params[0].Value.Name)
			assert.Equal(t, result[0].In, tt.args.params[0].Value.In)
			assert.Equal(t, result[0].Required, tt.args.params[0].Value.Required)
		})
	}
}

func TestSortParamsByPath(t *testing.T) {
	type args struct {
		path string
		in   []parameterDefinition
	}

	tests := []struct {
		name    string
		args    args
		output  []parameterDefinition
		wantErr bool
	}{
		{
			name:    "successful execution with in parameter as nil",
			args:    args{path: "/api/dashboards/db", in: nil},
			wantErr: false,
		},
		{
			name:    "successful execution with data",
			args:    args{path: "/api/teams/{teamId}/users/{userId}", in: paramDefinitionTestData},
			wantErr: false,
		},
		{
			name:    "unsuccessful execution: path X has Y positional parameters, but spec has K declared",
			args:    args{path: "/", in: paramDefinitionTestData},
			wantErr: true,
		},
		{
			name:    "unsuccessful execution: path refers to parameter which doesn't exist in specification",
			args:    args{path: "/i-just/made/this/path", in: paramDefinitionTestData},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run("test sortParamsByPath(): "+tt.name, func(t *testing.T) {
			_, err := sortParamsByPath(tt.args.path, tt.args.in)

			if (err != nil) != tt.wantErr {
				t.Errorf("error executing sortParamsByPath(). error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefineOperations(t *testing.T) {
	type args struct {
		templateURI string
	}

	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name:    "sad path: broken template that we are tampering",
			args:    args{templateURI: "https://raw.githubusercontent.com/blinkops/blink-grafana/master/grafana-openapi.yaml"},
			wantErr: "which doesn't exist in specification",
		},
		{
			name:    "happy path: valid template",
			args:    args{templateURI: "https://raw.githubusercontent.com/blinkops/blink-grafana/master/grafana-openapi.yaml"},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run("test DefineOperations(): "+tt.name, func(t *testing.T) {
			// first we reset the map.
			OperationDefinitions = map[string]*OperationDefinition{}

			// need to load some template for this test to work.
			// TBA: replace the http request with a static template, preferably inline and NOT a file.
			// because ideally, our tests should be self contained.
			// I use panic because the following are pre-reqs for the tests to start, they're not what I test.
			testTemplate := tt.args.templateURI
			loader := openapi3.NewLoader()
			loader.IsExternalRefsAllowed = true
			u, err := url.Parse(testTemplate)
			if err != nil {
				panic("unable to load openapi template uri")
			}

			openApi, err := loader.LoadFromURI(u)
			if err != nil {
				panic("unable to load openapi template")
			}

			// starting the test

			// let's do some template tampering to generate an error we want to have
			if tt.name != "happy path: valid template" {
				openApi.Paths["/api/org/users/{tampereddddd}"] = openApi.Paths["/api/org/users/{userId}"]
			}

			err = DefineOperations(openApi)
			if tt.wantErr != "" {
				require.NotNil(t, err, tt.name)
			} else {
				require.Nil(t, err, tt.name)
				assert.True(t, len(OperationDefinitions) > 0)
				assert.Contains(t, OperationDefinitions, "AddTeamMember")
			}
		})
	}
}

func TestGetPropertyByName(t *testing.T) {
	schemaByte := []byte(`{
 "description": "Folder details",
 "properties": {
  "dashboard": {
   "properties": {
    "id": {},
    "refresh": {
     "minLength": 1,
     "type": "string"
    },
    "schemaVersion": {
     "type": "number"
    },
    "tags": {
     "items": {},
     "type": "array"
    },
    "timezone": {
	"description": "my test description",
     "minLength": 1,
     "type": "string"
    },
    "title": {
     "minLength": 1,
     "type": "string"
    },
    "uid": {},
    "version": {
     "type": "number"
    }
   },
   "required": [
    "title",
    "tags",
    "timezone",
    "schemaVersion",
    "version",
    "refresh"
   ],
   "type": "object"
  },
  "folderId": {
   "type": "number"
  },
  "folderUid": {
   "minLength": 1,
   "type": "string"
  },
  "message": {
   "minLength": 1,
   "type": "string"
  },
  "overwrite": {
   "type": "boolean"
  }
 },
 "required": [
  "dashboard",
  "folderId",
  "folderUid",
  "message",
  "overwrite"
 ],
 "type": "object",
 "x-examples": {
  "example-1": {
   "dashboard": {
    "id": null,
    "refresh": "25s",
    "schemaVersion": 16,
    "tags": [
     "templated"
    ],
    "timezone": "browser",
    "title": "Production Overview",
    "uid": null,
    "version": 0
   },
   "folderId": 0,
   "folderUid": "nErXDvfCkzz",
   "message": "Made changes to xyz",
   "overwrite": false
  }
 }
}`)

	var schema *openapi3.Schema
	_ = json.Unmarshal(schemaByte, &schema)

	type args struct {
		propertyName string
	}

	tests := []struct {
		name   string
		args   args
		nilOutput bool
	}{
		{
			name: "Unexisting properly name - nil result",
			args: args{propertyName: "unexistingProperyName"},
			nilOutput: true,
		},
		{
			name: "Existing properly name - schema result",
			args: args{propertyName: "folderId"},
			nilOutput: false,
		},
	}
	for _, tt := range tests {
		t.Run("test GetPropertyByName(): "+tt.name, func(t *testing.T) {
			subPropertySchema := GetPropertyByName(tt.args.propertyName, schema)
			assert.Equal(t, (subPropertySchema == nil), tt.nilOutput)
		})
	}
}