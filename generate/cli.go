package gen

import (
	"github.com/urfave/cli/v2"
)

func GenerateMaskFile(c *cli.Context) error {
	return _generateMaskFile(
		c.String("file"),
		c.String("mask"),
		c.StringSlice("blacklist-params"),
		c.String("output"),
		c.Bool("filterParameters"),
		c.Bool("no-warnings"))
}

func GenerateReadme(c *cli.Context) error {
	return _GenerateReadme(
		c.String("name"),
		c.String("mask"),
		c.String("file"),
		c.String("custom-actions"))
}

// GenerateAction appends a single ParameterName to mask file.
func GenerateAction(c *cli.Context) error {
	return _generateAction(
		c.String("name"),
		c.String("file"),
		c.String("output"),
		c.StringSlice("blacklist-params"),
		c.String("interactive"))
}
