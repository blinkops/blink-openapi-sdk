package mask

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	"testing"
)

var (
	testData = `actions:
  AddTeamMember:
    parameters:
      teamId:
        alias: "Team ID"
      userId:
        alias: "User ID"

  InviteOrgMember:
    parameters:
      name:
        alias: "Name"
      loginOrEmail:
        alias: "Username/Email"
      role:
        alias: "Role"
      sendEmail:
        alias: "Send Email"

  RemoveOrgMember:
    parameters:
      userId:
        alias: "User ID"

  CreateFolder:
    parameters:
      uid:
        alias: "Folder ID (optional)"
      title:
        alias: "Folder Name"

  CreateDashboard:
    parameters:
      dashboard.uid:
        alias: "Dashboard Unique ID"
      dashboard.title:
        alias: "Dashboard Title"
      dashboard.tags:
        alias: "Dashboard Tags"
      dashboard.timezone:
        alias: "Dashboard Timezone"
      dashboard.schemaVersion:
        alias: "Dashboard Schema Version"
      dashboard.version:
        alias: "Dashboard Version"
      dashboard.refresh:
        alias: "Dashboard Refresh Interval"
      folderUid:
        alias: "Folder Unique ID"
      message:
        alias: "Commit Message"`
)

type MaskTestSuite struct {
	suite.Suite
}

// Need to populate the global variables.
func (suite *MaskTestSuite) SetupSuite() {
	if err := yaml.Unmarshal([]byte(testData), MaskData); err != nil {
		panic("unable to unmarshal test data")
	}
}

func (suite *MaskTestSuite) TestBuildActionAliasMap() {
	buildActionAliasMap()
	assert.Equal(suite.T(), len(reverseActionAliasMap), 0)
}

func (suite *MaskTestSuite) TestBuildParamAliasMap() {
	buildParamAliasMap()
	assert.Equal(suite.T(), len(reverseParameterAliasMap), 5)
	assert.Contains(suite.T(), reverseParameterAliasMap, "AddTeamMember")
	assert.Contains(suite.T(), reverseParameterAliasMap["AddTeamMember"], "Team ID")
	assert.Contains(suite.T(), reverseParameterAliasMap["AddTeamMember"], "User ID")
}

func (suite *MaskTestSuite) TestReplaceActionAlias() {
	input := "InviteOrgMember"
	result := ReplaceActionAlias(input)
	assert.Equal(suite.T(), result, input)
}

func (suite *MaskTestSuite) TestGetAction() {
	action := MaskData.GetAction("CreateFolder")
	assert.Equal(suite.T(), action.Alias, "")
	assert.Equal(suite.T(), len(action.Parameters), 2)
	assert.Equal(suite.T(), action.Parameters["uid"].Alias, "Folder ID (optional)")
}

func (suite *MaskTestSuite) TestGetParameter() {
	actionParameter := MaskData.GetParameter("CreateFolder", "title")
	assert.Equal(suite.T(), actionParameter.Alias, "Folder Name")
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestMaskSuite(t *testing.T) {
	suite.Run(t, new(MaskTestSuite))
}
