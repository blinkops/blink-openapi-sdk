package customact

import (
	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-sdk/plugin"
	log "github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CustomActTestSuite struct {
	suite.Suite
	actions CustomActions
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestPluginSuite(t *testing.T) {
	suite.Run(t, new(CustomActTestSuite))
}

func (suite *CustomActTestSuite) SetupSuite() {

	actions := map[string]ActionHandler{
		"CreateIssue": createIssue,
		"CreateIssueGzipped": createIssue,
	}

	suite.actions = CustomActions{
		Actions:           actions,
		ActionsFolderPath: "",
	}

}

func createIssue(_ *plugin.ActionContext, _ *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error) {
	return &plugin.ExecuteActionResponse{ErrorCode: consts.OK, Result: []byte("issue created")}, nil
}

func (suite *CustomActTestSuite) TestGetActions() {
	actions := suite.actions.GetActions()
	require.Equal(suite.T(), 1, len(actions))
	assert.Equal(suite.T(), "CreateIssue", actions[0].Name)
	assert.Equal(suite.T(), 5, len(actions[0].Parameters))
}

func (suite *CustomActTestSuite) TestHasAction() {
	assert.True(suite.T(), suite.actions.HasAction("CreateIssue"))
	assert.False(suite.T(), suite.actions.HasAction("NonExistingAction"))
}

func (suite *CustomActTestSuite) TestExecute() {
	actionContext := &plugin.ActionContext{}
	goodRequest := &plugin.ExecuteActionRequest{Name: "CreateIssue"}
	badRequest := &plugin.ExecuteActionRequest{Name: "NonExistingAction"}
	_, err := suite.actions.Execute(actionContext, goodRequest)
	assert.Nil(suite.T(), err)
	_, err = suite.actions.Execute(actionContext, badRequest)
	assert.NotNil(suite.T(), err)
}

func (suite *CustomActTestSuite) TestZipped() {
	err := os.Setenv("PROD", "true")
	require.Nil(suite.T(), err)

	_, err = exec.Command("gzip", "test.action.yaml").Output() // zip the file
	require.Nil(suite.T(), err)

	actions := suite.actions.GetActions()

	require.Equal(suite.T(), 1, len(actions))
	assert.Equal(suite.T(), "CreateIssue", actions[0].Name)
	assert.Equal(suite.T(), 5, len(actions[0].Parameters))

	err = os.Unsetenv("PROD")
	if err != nil {
		log.Error("Failed to unset environment var")
	}
}
