package plugin

import (
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	plugin_sdk "github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/connections"
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"

	"encoding/json"
	"testing"
)


var (
	myPlugin *openApiPlugin
)

func TestMain(t *testing.T) {
	myPlugin = &openApiPlugin{
		TestCredentialsFunc: func(ctx *plugin_sdk.ActionContext) (*plugin_sdk.CredentialsValidationResponse, error) {
			return &plugin_sdk.CredentialsValidationResponse{}, nil
		},
		ValidateResponse: func(response Result) (bool, []byte) {
			return true, nil
		},
		actions: []plugin_sdk.Action{
			plugin_sdk.Action{
				Name:        "AddTeamMember",
				Description: "AddTeamMember",
				Enabled:     true,
				EntryPoint:  "/api/teams/{teamId}/members",
				Parameters: map[string]plugin_sdk.ActionParameter{
					"Team ID": plugin_sdk.ActionParameter{
						Type: "integer",
						Description: "Team ID to add member to",
						Placeholder: "",
						Required: true,
						Default: "",
						Pattern: "",
						Options: []string{},
					},
				},
				Output: nil,
			},
			plugin_sdk.Action{
				Name:        "InviteOrgMember",
				Description: "InviteOrgMember",
				Enabled:     true,
				EntryPoint:  "/api/org/invites",
				Parameters: map[string]plugin_sdk.ActionParameter{
					"Name": plugin_sdk.ActionParameter{
						Type: "integer",
						Description: "User to invite",
						Placeholder: "",
						Required: true,
						Default: "",
						Pattern: "",
						Options: []string{},
					},
				},
				Output: nil,
			},
		},
		description: plugin_sdk.Description{
			Name: "my-fake-plugin",
			Description: "my-fake-plugin-yet-again",
			Tags: []string{"fake-plugin"},
			Provider: "test",
			Connections: map[string]connections.Connection{
				"test": {
					Name:      "test",
					Fields:    nil,
					Reference: "test",
				},
			},
			Version: "1.0.0",
		},
	}
}

func TestActionExists(t *testing.T) {
	type args struct {
		action string
	}

	tests := []struct {
		name    string
		args    args
		expectedResult  bool
	}{
		{
			name:           "Existing Action",
			args:           args{action: myPlugin.actions[0].Name},
			expectedResult: true,
		},
		{
			name:           "NON-Existing Action",
			args:           args{action: "my-nonexisting-action"},
			expectedResult: false,
		},
	}
	for _, tt := range tests {
		t.Run("test ActionExist(): "+tt.name, func(t *testing.T) {
			result := myPlugin.ActionExist(tt.args.action)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestExecuteRequest(t *testing.T) {
	// This is a test http server.
	// I could change the function's signature but decided not to, since I can also pass a fake URL.
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(`{"test":"wa wa"}`)) // nolint
	}))
	defer func() { testServer.Close() }()

	type args struct {
		providerName string
		httpreq      *http.Request
		cns          connections.ConnectionInstance
	}

	u, _ := url.Parse(testServer.URL)
	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name: "sad path: missing in action context",
			args: args{providerName: "some-bad-provider",
				httpreq: &http.Request{Method: "POST",
					URL: &url.URL{Scheme: "http",
					Host: u.Host, Path: u.Path},
					Header: map[string][]string{"Authorization": []string{"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "missing in action context",
		},
		{
			name: "sad path: no such host",
			args: args{providerName: "test",
				httpreq: &http.Request{Method: "POST",
					URL: &url.URL{Scheme: "http",
					Host: "some-non-existing-host.com", Path: "some-fake-path"},
					Header: map[string][]string{"Authorization": []string{"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "no such host",
		},
		{
			name: "sad path: mismatch between schemes (http response for https server)",
			args: args{providerName: "test",
				httpreq: &http.Request{Method: "POST",
					URL: &url.URL{Scheme: "https",
						Host: u.Host, Path: u.Path},
					Header: map[string][]string{"Authorization": []string{"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "server gave HTTP response to HTTPS client",
		},
		{
			name: "happy path",
			args: args{providerName: "test",
				httpreq: &http.Request{Method: "POST",
					URL: &url.URL{Scheme: "http",
					Host: u.Host, Path: u.Path},
					Header: map[string][]string{"Authorization": []string{"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run("test ExecuteRequest(): "+tt.name, func(t *testing.T) {
			cns := map[string]connections.ConnectionInstance{}
			cns["test"] = tt.args.cns
			ctx := plugin_sdk.NewActionContext(map[string]interface{}{}, cns)
			result, err := ExecuteRequest(ctx, tt.args.httpreq, tt.args.providerName, nil, nil, 30)

			if tt.wantErr != "" {
				require.NotNil(t, err, tt.name)
				assert.Contains(t, err.Error(), tt.wantErr, tt.name)
				assert.NotEqual(t, result.StatusCode, 200)
			} else {
				assert.Equal(t, result.StatusCode, 200)
			}
		})
	}
}

func TestParseActionRequest(t *testing.T) {
	// again this server is required because of a live http request made
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(`{"test":"wa wa"}`)) //nolint
	}))
	defer func() { testServer.Close() }()

	cns := map[string]connections.ConnectionInstance{}
	cns["test"] = connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}
	ctx := plugin_sdk.NewActionContext(map[string]interface{}{}, cns)

	// handlers.DefineOperations populates a global variable (OperationDefinitions) that is REQUIRED for this run.
	// The only convenient option for populating this var is to load an api from file
	// the other one - loading from file is just too inconvenient
	openApi, err := loadOpenApi("https://raw.githubusercontent.com/blinkops/blink-grafana/master/grafana-openapi.yaml")
	if err != nil {
		panic("unable to load openapi template")
	}
	err = handlers.DefineOperations(openApi)
	if err != nil {
		panic("unable to prepare DefineOperations() for test")
	}

	type args struct {
		executeActionRequest *plugin_sdk.ExecuteActionRequest
	}

	tests := []struct {
		name    string
		args    args
		wantErr  string
	}{
		{
			name:           "sad path: No such method",
			args:           args{executeActionRequest: &plugin_sdk.ExecuteActionRequest{Name:"AddTeamMemberAAAAAAAAAAAAAAAA", Parameters:map[string]string{}}},
			wantErr: "No such method",
		},
		{
			name:           "happy path: all good!",
			args:           args{executeActionRequest: &plugin_sdk.ExecuteActionRequest{Name:"InviteOrgMember", Parameters:map[string]string{"name": "123"}}},
			wantErr: "",
		},
		{
			name:           "happy path: but with warning - Invalid request body param passed. This command has no body params at all. only path params",
			args:           args{executeActionRequest: &plugin_sdk.ExecuteActionRequest{Name:"AddTeamMember", Parameters:map[string]string{"fake_command": "123"}}},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run("test parseActionRequest(): "+tt.name, func(t *testing.T) {
			httpreq, err := myPlugin.parseActionRequest(ctx, tt.args.executeActionRequest)
			if tt.wantErr != "" {
				require.NotNil(t, err, tt.name)
				assert.Contains(t, err.Error(), tt.wantErr, tt.name)
			} else {
				require.NoError(t, err, tt.name)
				assert.NotNil(t, httpreq.Host)
				assert.Equal(t, httpreq.Method, "POST")
			}
		})
	}
}


func TestExecuteAction(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(`{"REQUEST_URL":"test.com"}`)) //nolint
	}))

	defer func() { testServer.Close() }()
	cns := map[string]connections.ConnectionInstance{}
	cns["test"] = connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}
	ctx := plugin_sdk.NewActionContext(map[string]interface{}{}, cns)

	executeActionRequest := &plugin_sdk.ExecuteActionRequest{Name:"InviteOrgMember", Parameters:map[string]string{"name": "123"}}
	executeActionResponse, err := myPlugin.ExecuteAction(ctx, executeActionRequest)

	assert.Nil(t, err)
	assert.Equal(t, executeActionResponse.ErrorCode, int64(0))
}


func TestHandleBodyParams(t *testing.T) {
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
	parentPath := ""
	action := myPlugin.actions[0]

	handleBodyParams(schema, parentPath, &action)

	assert.Equal(t, len(myPlugin.actions[0].Parameters), 13)
	assert.Contains(t, myPlugin.actions[0].Parameters, "dashboard.id")
	assert.Contains(t, myPlugin.actions[0].Parameters, "dashboard.timezone")
	assert.Equal(t, myPlugin.actions[0].Parameters["dashboard.timezone"].Description, "my test description")
	assert.Equal(t, myPlugin.actions[0].Parameters["dashboard.timezone"].Required, true)
}
