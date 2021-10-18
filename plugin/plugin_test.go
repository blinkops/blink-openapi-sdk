package plugin

import (
	"fmt"
	"github.com/blinkops/blink-openapi-sdk/plugin/handlers"
	plugin_sdk "github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/connections"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"

	"encoding/json"
	"testing"
)

var (
	myPlugin *OpenApiPlugin
	schemaByte = []byte(`{
 "description": "Folder details",
 "properties": {
  "dashboard": {
   "description": "dashboard description",
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
)

type PluginTestSuite struct {
	suite.Suite
}

func (suite *PluginTestSuite) BeforeTest(_, _ string) {
	suite.defineOperations()
}

func (suite *PluginTestSuite) AfterTest(_, _ string) {
	suite.resetOperations()
}

func (suite *PluginTestSuite) SetupSuite() {
	myPlugin = &OpenApiPlugin{
		actions: []plugin_sdk.Action{
			{
				Name:        "AddTeamMember",
				Description: "AddTeamMember",
				Enabled:     true,
				EntryPoint:  "/api/teams/{teamId}/members",
				Parameters: map[string]plugin_sdk.ActionParameter{
					"Team ID": {
						Type:        "integer",
						Description: "Team ID to add member to",
						Placeholder: "",
						Required:    true,
						Default:     "",
						Pattern:     "",
						Options:     []string{},
					},
				},
				Output: nil,
			},
			{
				Name:        "InviteOrgMember",
				Description: "InviteOrgMember",
				Enabled:     true,
				EntryPoint:  "/api/org/invites",
				Parameters: map[string]plugin_sdk.ActionParameter{
					"Name": {
						Type:        "integer",
						Description: "User to invite",
						Placeholder: "",
						Required:    true,
						Default:     "",
						Pattern:     "",
						Options:     []string{},
					},
				},
				Output: nil,
			},
		},
		description: plugin_sdk.Description{
			Name:        "my-fake-plugin",
			Description: "my-fake-plugin-yet-again",
			Tags:        []string{"fake-plugin"},
			Provider:    "test",
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

func (suite *PluginTestSuite) TestActionExists() {
	type args struct {
		action string
	}

	tests := []struct {
		name           string
		args           args
		expectedResult bool
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
		suite.T().Run("test ActionExist(): "+tt.name, func(t *testing.T) {
			result := myPlugin.ActionExist(tt.args.action)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func (suite *PluginTestSuite) TestExecuteRequest() {
	// This is a test http server.
	// I could change the function's signature but decided not to, since I can also pass a fake URL.
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte(`{"Username":"sawit", "Password": "wa"}`))
		if err != nil {
			assert.Nil(suite.T(), err)
		} // nolint
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
					Header: map[string][]string{"Authorization": {"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "missing in action context",
		},
		{
			name: "sad path: no such host",
			args: args{providerName: "test",
				httpreq: &http.Request{Method: "POST",
					URL: &url.URL{Scheme: "http",
						Host: "some-non-existing-host.com", Path: "some-fake-path"},
					Header: map[string][]string{"Authorization": {"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "no such host",
		},
		{
			name: "sad path: mismatch between schemes (http response for https server)",
			args: args{providerName: "test",
				httpreq: &http.Request{Method: "POST",
					URL: &url.URL{Scheme: "https",
						Host: u.Host, Path: u.Path},
					Header: map[string][]string{"Authorization": {"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "server gave HTTP response to HTTPS client",
		},
		{
			name: "happy path",
			args: args{providerName: "test",
				httpreq: &http.Request{Method: "POST",
					URL: &url.URL{Scheme: "http",
						Host: u.Host, Path: u.Path},
					Header: map[string][]string{"Authorization": {"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "",
		},
		{
			name: "happy path with bearer auth",
			args: args{providerName: "test",
				httpreq: &http.Request{Method: "POST",
					URL: &url.URL{Scheme: "http",
						Host: u.Host, Path: u.Path},
					Header: map[string][]string{"Authorization": {"test1", "test2"}}},
				cns: connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		suite.T().Run("test ExecuteRequest(): "+tt.name, func(t *testing.T) {
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

func (suite *PluginTestSuite) TestParseActionRequest() {
	// again this server is required because of a live http request made
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte(`{"test":"wa wa"}`))
		if err != nil {
			assert.Nil(suite.T(), err)
		} //nolint
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
		wantErr string
	}{
		{
			name:    "sad path: No such method",
			args:    args{executeActionRequest: &plugin_sdk.ExecuteActionRequest{Name: "AddTeamMemberAAAAAAAAAAAAAAAA", Parameters: map[string]string{}}},
			wantErr: "No such method",
		},
		{
			name:    "happy path: all good!",
			args:    args{executeActionRequest: &plugin_sdk.ExecuteActionRequest{Name: "InviteOrgMember", Parameters: map[string]string{"name": "123"}}},
			wantErr: "",
		},
		{
			name:    "happy path: but with warning - Invalid request body param passed. This command has no body params at all. only path params",
			args:    args{executeActionRequest: &plugin_sdk.ExecuteActionRequest{Name: "AddTeamMember", Parameters: map[string]string{"fake_command": "123"}}},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		suite.T().Run("test parseActionRequest(): "+tt.name, func(t *testing.T) {
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

func (suite *PluginTestSuite) TestExecuteAction() {
	address := "127.0.0.1:8888"
	l, err := net.Listen("tcp", address)
	if err != nil {
		suite.T().Error("could not start listener for httptest server")
	}

	handler := http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte(fmt.Sprintf(`{"REQUEST_URL":"http://%s"}`, address)))
		if err != nil {
			assert.Nil(suite.T(), err)
		} //nolint
	})

	testServer := httptest.NewUnstartedServer(handler)
	testServer.Listener = l
	testServer.Start()

	defer testServer.Listener.Close()
	defer func() { testServer.Close() }()
	cns := map[string]connections.ConnectionInstance{}
	cns["test"] = connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}
	ctx := plugin_sdk.NewActionContext(map[string]interface{}{}, cns)

	executeActionRequest := &plugin_sdk.ExecuteActionRequest{Name: "InviteOrgMember", Parameters: map[string]string{"name": "123"}}
	executeActionResponse, err := myPlugin.ExecuteAction(ctx, executeActionRequest)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), executeActionResponse.ErrorCode, int64(0))
}

func (suite *PluginTestSuite) TestHandleBodyParams() {
	var schema *openapi3.Schema
	_ = json.Unmarshal(schemaByte, &schema)
	parentPath := ""
	action := myPlugin.actions[0]

	handleBodyParams(schema, parentPath, &action)

	assert.Equal(suite.T(), len(myPlugin.actions[0].Parameters), 13)
	assert.Contains(suite.T(), myPlugin.actions[0].Parameters, "dashboard.id")
	assert.Contains(suite.T(), myPlugin.actions[0].Parameters, "dashboard.timezone")
	assert.Equal(suite.T(), myPlugin.actions[0].Parameters["dashboard.timezone"].Description, "my test description")
	assert.Equal(suite.T(), myPlugin.actions[0].Parameters["dashboard.timezone"].Required, true)
}

func (suite *PluginTestSuite) TestParseActionParam() {
	var schema *openapi3.Schema
	_ = json.Unmarshal(schemaByte, &schema)
	paramName := "dashboard"
	pathParam := schema.Properties[paramName]

	actionParam := parseActionParam("test", &paramName, pathParam, false, pathParam.Value.Description)

	assert.False(suite.T(), actionParam.Required)
	assert.Equal(suite.T(), actionParam.Description, "dashboard description")
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestPluginSuite(t *testing.T) {
	suite.Run(t, new(PluginTestSuite))
}

func (suite *PluginTestSuite) defineOperations() {
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
}

func (suite *PluginTestSuite) resetOperations() {
	handlers.OperationDefinitions = map[string]*handlers.OperationDefinition{}
}
