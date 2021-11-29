package plugin

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	plugin_sdk "github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/connections"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *PluginTestSuite) TestSetAuthenticationHeaders() {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte(`{"Username":"sawit", "Password": "wa", "Something": "wawa"}`))
		if err != nil {
			require.Nil(suite.T(), err)
		}
	}))
	defer func() { testServer.Close() }()

	cns := map[string]*connections.ConnectionInstance{}
	cns["test"] = &connections.ConnectionInstance{VaultUrl: testServer.URL, Name: "test", Id: "lewl", Token: "1234"}
	ctx := plugin_sdk.NewActionContext(map[string]interface{}{}, cns)

	type args struct {
		httpreq *http.Request
	}

	u, _ := url.Parse(testServer.URL)

	tests := []struct {
		name string
		args args
	}{
		{
			name: "happy path",
			args: args{httpreq: &http.Request{
				Method: "POST",
				URL: &url.URL{
					Scheme: "http",
					Host:   u.Host, Path: u.Path,
				},
				Header: map[string][]string{},
			}},
		},
	}
	for _, tt := range tests {
		suite.T().Run("test parseActionRequest(): "+tt.name, func(t *testing.T) {
			connection, err := ctx.GetCredentials("test")
			require.Nil(t, err)
			err = setAuthenticationHeaders(connection, tt.args.httpreq, nil, nil)
			require.Nil(t, err)
			assert.Contains(t, tt.args.httpreq.Header, "Authorization")
			splitSlice := strings.Split(tt.args.httpreq.Header["Authorization"][0], " ")
			assert.Equal(t, len(splitSlice), 2)
			userAndPassEncoded := splitSlice[1]
			_, err = base64.StdEncoding.DecodeString(userAndPassEncoded)
			require.Nil(t, err)
		})
	}
}
