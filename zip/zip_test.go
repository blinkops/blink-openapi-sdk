package zip

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"testing"
)

type ZipTestSuite struct {
	suite.Suite
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestPluginSuite(t *testing.T) {
	suite.Run(t, new(ZipTestSuite))
}

func (suite *ZipTestSuite) TestReadGzipFile() {
	type args struct {
		filePath string
	}

	var emptyFile []byte
	const gzipFile = "mask_test.yaml.gz"
	const normalFile = "../mask/mask_test.yaml"

	fileContent, err := ioutil.ReadFile(normalFile)
	require.Nil(suite.T(), err)

	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr string
	}{
		{
			name: "read fileContent",
			args: args{
				filePath: gzipFile,
			},
			want:    fileContent,
			wantErr: "",
		},
		{
			name: "same fileContent",
			args: args{
				filePath: normalFile,
			},
			want:    emptyFile,
			wantErr: "",
		},
	}
	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			got, err := ReadGzipFile(tt.args.filePath)
			if tt.wantErr != "" {
				require.NotNil(t, err, tt.name)

				assert.Contains(t, err.Error(), tt.wantErr, tt.name)
			}

			require.Equal(t, got, tt.want)
		})
	}
}
