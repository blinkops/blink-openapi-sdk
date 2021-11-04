package main

import (
	"github.com/blinkops/blink-openapi-sdk/generate"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
)

func main() {
	app := &cli.App{

		Commands: []*cli.Command{
			{
				Name:    "generate",
				Aliases: []string{"gen"},

				Subcommands: []*cli.Command{
					{
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "file",
								Aliases:  []string{"f"},
								Value:    "",
								Usage:    "openApi file",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "name",
								Aliases:  []string{"n"},
								Value:    "",
								Usage:    "plugin name",
								Required: true,
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
								Name:     "file",
								Aliases:  []string{"f"},
								Value:    "",
								Usage:    "openApi file name",
								Required: true,
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
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
