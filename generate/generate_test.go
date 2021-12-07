package gen

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/blinkops/blink-openapi-sdk/mask"
	sdkPlugin "github.com/blinkops/blink-sdk/plugin"
	"github.com/stretchr/testify/suite"
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
		act              sdkPlugin.Action
		filterParameters bool
	}
	tests := []struct {
		name string
		args args
		want GeneratedAction
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
		want    []GeneratedAction
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
		act  sdkPlugin.Action
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "No prefix",
			args: args{
				act: sdkPlugin.Action{
					Parameters: map[string]sdkPlugin.ActionParameter{
						"A": {},
						"B": {},
						"C": {},
					},
				},
				name: "A",
			},
			want: false,
		},
		{
			name: "Has prefix",
			args: args{
				act: sdkPlugin.Action{
					Parameters: map[string]sdkPlugin.ActionParameter{
						"A":   {},
						"A.B": {},
						"A.C": {},
					},
				},
				name: "A",
			},
			want: true,
		},
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
		{
			name: "not in array",
			args: args{
				name:  "bruh",
				array: []string{"a", "b", "c"},
			},
			want: false,
		},
		{
			name: "in array",
			args: args{
				name:  "a",
				array: []string{"a", "b", "c"},
			},
			want: true,
		},
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
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "underscore",
			str:  "team_slug",
			want: "Team Slug",
		},
		{
			name: "brackets",
			str:  "amogus[]",
			want: "Amogus",
		},
		{
			name: "id",
			str:  "user_id",
			want: "User ID",
		},
		{
			name: "IDs",
			str:  "channel_ids",
			want: "Channel IDs",
		},
		{
			name: "long name",
			str:  "bruh_bruh_bruh_bruh",
			want: "Bruh Bruh Bruh Bruh",
		},
		{
			name: "gcp like naming",
			str:  "service.s3_lol",
			want: "Service S3 Lol",
		},
		{
			name: "upper case this",
			str:  "url_id_ids ip_ssl",
			want: "URL ID IDs IP SSL",
		},
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			if got := genAlias(tt.str); got != tt.want {
				t.Errorf("genAlias() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *GenerateTestSuite) Test_replaceOldActionWithNew() {
	type args struct {
		actions   []GeneratedAction
		newAction GeneratedAction
	}
	tests := []struct {
		name string
		args args
		want []GeneratedAction
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
		{
			name: "generate mask",
			args: args{
				templateStr: YAMLTemplate,
				obj: []GeneratedAction{
					{
						Name:  "actions_first",
						Alias: "First",
						Parameters: map[string]GeneratedParameter{"name": {
							Alias:    "Name",
							Required: true,
							Index:    0,
						}},
					},
				},
			},
			wantF: `actions:
  actions_first:
    alias: First
    parameters:
      name:
        alias: "Name"
        required: true
        index: 1`,
			wantErr: false,
		},
		{
			name: "invalid template",
			args: args{
				templateStr: `{{range $Action := .}}
  {{$Action.dsfsdf }}:`,
				obj: []GeneratedAction{
					{
						Name:  "actions_first",
						Alias: "First",
						Parameters: map[string]GeneratedParameter{"name": {
							Alias:    "Name",
							Required: true,
							Index:    0,
						}},
					},
				},
			},
			wantF:   "",
			wantErr: true,
		},
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
