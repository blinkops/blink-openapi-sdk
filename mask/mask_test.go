package mask

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type MaskTestSuite struct {
	suite.Suite
	Mask Mask
}

// Need to populate the global variables.
func (suite *MaskTestSuite) SetupSuite() {
	mask, err := ParseMask("mask_test.yaml")
	if err != nil {
		panic("unable to parse mask from test data")
	}
	suite.Mask = mask
}

func (suite *MaskTestSuite) TestBuildActionAliasMap() {
	suite.Mask.buildActionAliasMap()
	assert.Equal(suite.T(), len(suite.Mask.ReverseActionAliasMap), 0)
}

func (suite *MaskTestSuite) TestBuildParamAliasMap() {
	suite.Mask.buildParamAliasMap()
	reverseParameterAliasMap := suite.Mask.ReverseParameterAliasMap
	assert.Equal(suite.T(), len(reverseParameterAliasMap), 5)
	assert.Contains(suite.T(), reverseParameterAliasMap, "AddTeamMember")
	assert.Contains(suite.T(), reverseParameterAliasMap["AddTeamMember"], "Team ID")
	assert.Contains(suite.T(), reverseParameterAliasMap["AddTeamMember"], "User ID")
}

func (suite *MaskTestSuite) TestReplaceActionAlias() {
	input := "InviteOrgMember"
	result := suite.Mask.ReplaceActionAlias(input)
	assert.Equal(suite.T(), result, input)
}

func (suite *MaskTestSuite) TestGetAction() {
	action := suite.Mask.GetAction("CreateFolder")
	assert.Equal(suite.T(), action.Alias, "")
	assert.Equal(suite.T(), len(action.Parameters), 2)
	assert.Equal(suite.T(), action.Parameters["uid"].Alias, "Folder ID (optional)")
}

func (suite *MaskTestSuite) TestGetParameter() {
	actionParameter := suite.Mask.GetParameter("CreateFolder", "title")
	assert.Equal(suite.T(), actionParameter.Alias, "Folder Name")
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestMaskSuite(t *testing.T) {
	suite.Run(t, new(MaskTestSuite))
}
