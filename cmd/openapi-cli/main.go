package main

import (
	"os"
	"regexp"
	"strings"

	"github.com/blinkops/blink-openapi-sdk/generate"
	log "github.com/sirupsen/logrus"
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

	return "", nil
}

func main() {
	OpenAPIFile, err := getOpenapiDefaultFile()
	if err != nil {
		log.Fatal(err)
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
						},
						Name:    "readme",
						Aliases: []string{"md", "r"},
						Usage:   "generate readme.md for openapi plugins",
						Action:  gen.GenerateMarkdown,
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
								Aliases:     []string{"o"},
								Value:       "mask.yaml",
								Usage:       "name of the output mask file",
								DefaultText: "mask.yaml",
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
								Name:        "action",
								Aliases:     []string{"act"},
								Value:       "",
								Usage:       "name of the action you want to generate",
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
								Aliases:     []string{"o"},
								Value:       "mask.yaml",
								Usage:       "name of the output mask file",
								DefaultText: "mask.yaml",
							},
						},
						Name:    "action",
						Aliases: []string{"act"},
						Usage:   "generate one action in mask file",
						Action:  gen.GenerateAction,
					},
				},
			},
		},
	}

	if err = app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
