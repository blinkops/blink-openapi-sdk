package gen

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/blinkops/blink-openapi-sdk/mask"
	"github.com/blinkops/blink-sdk/plugin"
	sdkPlugin "github.com/blinkops/blink-sdk/plugin"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli/v2"
)

type GenerateTestSuite struct {
	suite.Suite
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestPluginSuite(t *testing.T) {
	suite.Run(t, new(GenerateTestSuite))
}

func (suite *GenerateTestSuite) TestFilterActionsByOperationName() {
	actions := []sdkPlugin.Action{
		{
			Name:        "AddTeamMember",
			Description: "AddTeamMember",
			Enabled:     true,
			EntryPoint:  "/api/teams/{teamId}/members",
			Parameters: map[string]sdkPlugin.ActionParameter{
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
			Parameters: map[string]sdkPlugin.ActionParameter{
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
	}

	type args struct {
		operationName string
		actions       []sdkPlugin.Action
	}
	tests := []struct {
		name string
		args args
		want *sdkPlugin.Action
	}{
		{
			name: "Get action",
			args: args{"InviteOrgMember", actions},
			want: &actions[1],
		},
		{
			name: "no such action",
			args: args{"BruhBruh", actions},
			want: nil,
		},
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := FilterActionsByOperationName(tt.args.operationName, tt.args.actions); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterActionsByOperationName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) TestFilterMaskedParameters() {
	type args struct {
		maskedAct        *mask.MaskedAction
		act              plugin.Action
		filterParameters bool
	}
	tests := []struct {
		name string
		args args
		want EnhancedAction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := FilterMaskedParameters(tt.args.maskedAct, tt.args.act, tt.args.filterParameters); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterMaskedParameters() = %v, want %v", got, tt.want)
			}
		})
	}
}


func (suite *GenerateTestSuite) TestGenerateMarkdown() {
	type args struct {
		c *cli.Context
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if err := GenerateMarkdown(tt.args.c); (err != nil) != tt.wantErr {
				t.Errorf("GenerateMarkdown() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func (suite *GenerateTestSuite) TestGenerateMaskFile() {
	type args struct {
		c *cli.Context
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if err := GenerateMaskFile(tt.args.c); (err != nil) != tt.wantErr {
				t.Errorf("GenerateMaskFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func (suite *GenerateTestSuite) TestGetMaskedActions() {
	type args struct {
		maskFile         string
		actions          []sdkPlugin.Action
		blacklistParams  []string
		filterParameters bool
	}
	tests := []struct {
		name    string
		args    args
		want    []EnhancedAction
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			got, err := GetMaskedActions(tt.args.maskFile, tt.args.actions, tt.args.blacklistParams, tt.args.filterParameters)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMaskedActions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMaskedActions() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) TestIsPrefix() {
	type args struct {
		act  plugin.Action
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := IsPrefix(tt.args.act, tt.args.name); got != tt.want {
				t.Errorf("IsPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) TestStringInSlice() {
	type args struct {
		name  string
		array []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := StringInSlice(tt.args.name, tt.args.array); got != tt.want {
				t.Errorf("StringInSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) Test_genAlias() {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := genAlias(tt.args.str); got != tt.want {
				t.Errorf("genAlias() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) Test_newAction() {
	type args struct {
		act plugin.Action
	}
	tests := []struct {
		name string
		args args
		want EnhancedAction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := newCliAction(tt.args.act); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newCliAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) Test_newParameter() {
	type args struct {
		a map[string]sdkPlugin.ActionParameter
	}
	tests := []struct {
		name string
		args args
		want map[string]Parameter
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := newParameter(tt.args.a); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newParameter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) Test_replaceOldActionWithNew() {
	type args struct {
		actions   []EnhancedAction
		newAction EnhancedAction
	}
	tests := []struct {
		name string
		args args
		want []EnhancedAction
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := replaceOldActionWithNew(tt.args.actions, tt.args.newAction); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("replaceOldActionWithNew() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) Test_runTemplate() {


	type args struct {
		templateStr string
		obj         interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantF   string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			f := &bytes.Buffer{}
			err := runTemplate(f, tt.args.templateStr, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("runTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotF := f.String(); gotF != tt.wantF {
				t.Errorf("runTemplate() gotF = %v, want %v", gotF, tt.wantF)
			}
		})
	}
}

func Test__generateAction(t *testing.T) {
	type args struct {
		actionName     string
		OpenApiFile    string
		outputFileName string
		paramBlacklist []string
		isInteractive  string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := _generateAction(tt.args.actionName, tt.args.OpenApiFile, tt.args.outputFileName, tt.args.paramBlacklist, tt.args.isInteractive); (err != nil) != tt.wantErr {
				t.Errorf("_generateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}