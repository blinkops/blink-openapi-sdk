package main

import (
	"fmt"
	gen "github.com/blinkops/blink-openapi-sdk/generate"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"github.com/urfave/cli/v2"
)

func getOpenapiDefaultFile() (string, error) {
	file, err := os.Open("./")
	if err != nil {
		return "", err
	}

	defer file.Close()

	names, err := file.Readdirnames(0)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`.*-openapi.yaml`)
	for _, name := range names {
		if val := re.Find([]byte(name)); val != nil {
			return string(val), nil
		}
	}

	return "", errors.New("Could not find an openAPI file in this directory.\n(for the file to be automagically detected use this pattern [.*-openapi.yaml])")
}

func main() {
	OpenAPIFile, err := getOpenapiDefaultFile()
	if err != nil {
		fmt.Println(err)
	}

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "generate",
				Aliases: []string{"gen"},

				Subcommands: []*cli.Command{
					{
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "file",
								Aliases: []string{"f"},
								Value:   OpenAPIFile,
								Usage:   "openApi file",
							},
							&cli.StringFlag{
								Name:    "name",
								Aliases: []string{"n"},
								Value:   strings.Split(OpenAPIFile, "-")[0],
								Usage:   "plugin name",
							},
							&cli.StringFlag{
								Name:        "mask",
								Aliases:     []string{"m"},
								Value:       "mask.yaml",
								Usage:       "mask file name",
								DefaultText: "mask.yaml",
							},
							&cli.StringFlag{
								Name:        "custom-actions",
								Aliases:     []string{"ca"},
								Usage:       "the path to the custom actions directory",
								Value:       "./custom_actions/actions",
								DefaultText: "./custom_actions/actions",
							},
						},
						Name:    "readme",
						Aliases: []string{"md", "r"},
						Usage:   "generate readme.md for openapi plugins",
						Action:  gen.GenerateReadme,
					},
					{
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "file",
								Aliases: []string{"f"},
								Value:   OpenAPIFile,
								Usage:   "openApi file name",
							},
							&cli.StringFlag{
								Name:        "mask",
								Aliases:     []string{"m"},
								Value:       "",
								Usage:       "mask file to regenerate a mask from",
								DefaultText: "",
							},
							&cli.StringFlag{
								Name:        "output",
								Aliases:     []string{"o", "out"},
								Value:       "mask.yaml",
								Usage:       "name of the output mask file",
								DefaultText: "mask.yaml",
							},
							&cli.StringSliceFlag{
								Name:    "blacklist-params",
								Aliases: []string{"exclude", "param-blacklist"},
								Usage:   "parameters you don't wish to generate across all actions.",
							},
							&cli.BoolFlag{
								Name:        "no-warnings",
								Value:       false,
								Usage:       "dont get warning messages",
								DefaultText: "false",
							},

							&cli.BoolFlag{
								Name:        "filterParameters",
								Usage:       "set to false if you dont wish to keep the original mask parameters",
								Value:       true,
								DefaultText: "true",
							},
						},
						Name:    "mask",
						Aliases: []string{"m"},
						Usage:   "generate mask.yaml for openapi plugins",
						Action:  gen.GenerateMaskFile,
					},
					{
						Flags: []cli.Flag{
							&cli.StringFlag{
								Required:    true,
								Name:        "name",
								Aliases:     []string{"act", "action-name"},
								Value:       "",
								Usage:       "The operation ID of the action you want to generate (from the openAPI file).",
								DefaultText: "",
							},
							&cli.StringFlag{
								Name:    "file",
								Aliases: []string{"f"},
								Value:   OpenAPIFile,
								Usage:   "openApi file name",
							},
							&cli.StringFlag{
								Name:        "output",
								Aliases:     []string{"o", "out"},
								Value:       "mask.yaml",
								Usage:       "name of the output mask file",
								DefaultText: "mask.yaml",
							},
							&cli.StringSliceFlag{
								Name:    "blacklist-params",
								Aliases: []string{"exclude", "param-blacklist"},
								Usage:   "parameters you don't wish to generate.",
							},
							&cli.BoolFlag{
								Name:    "interactive",
								Aliases: []string{"i"},
								Usage:   "the cli will prompt the user to choose parameters",
							},
						},
						Name:    "action",
						Aliases: []string{"act"},
						Usage:   "generate one action in mask file",
						Action:  gen.GenerateAction,
					},
				},
			},
			{
				Name:    "fix-mask",
				Aliases: []string{"fm"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "path",
						Aliases: []string{"p"},
						Value:   "mask.yaml",
						Usage:   "mask file path",
					},
				},
				Usage:  "fix mask file with nested params separated by . to be separated by __",
				Action: gen.FixMask,
			},
		},
	}

	if err = app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
