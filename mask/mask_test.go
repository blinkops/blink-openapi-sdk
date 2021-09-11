package mask

import (
	"github.com/stretchr/testify/assert"
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

// Need to populate the global variables.
func TestMain(t *testing.T) {
	if err := yaml.Unmarshal([]byte(testData), MaskData); err != nil {
		panic("unable to unmarshal test data")
	}

}


func TestBuildActionAliasMap(t *testing.T) {
	buildActionAliasMap()
	assert.Equal(t, len(reverseActionAliasMap), 0)
}

func TestBuildParamAliasMap(t *testing.T) {
	buildParamAliasMap()
	assert.Equal(t, len(reverseParameterAliasMap), 5)
	assert.Contains(t, reverseParameterAliasMap, "AddTeamMember")
	assert.Contains(t, reverseParameterAliasMap["AddTeamMember"], "Team ID")
	assert.Contains(t, reverseParameterAliasMap["AddTeamMember"], "User ID")
}

func TestReplaceActionAlias(t *testing.T) {
	input := "InviteOrgMember"
	result := ReplaceActionAlias(input)
	assert.Equal(t, result, input)
}

func TestGetAction(t *testing.T) {
	action := MaskData.GetAction("CreateFolder")
	assert.Equal(t, action.Alias, "")
	assert.Equal(t, len(action.Parameters), 2)
	assert.Equal(t, action.Parameters["uid"].Alias, "Folder ID (optional)")
}

func TestGetParameter(t *testing.T) {
	actionParameter := MaskData.GetParameter("CreateFolder", "title")
	assert.Equal(t, actionParameter.Alias, "Folder Name")
}


/*
func TestReplaceActionParametersAliases(t *testing.T) {
	result := ReplaceActionParametersAliases("InviteOrgMember", )
}
*/