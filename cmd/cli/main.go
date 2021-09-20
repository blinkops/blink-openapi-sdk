package main

import (
	"github.com/blinkops/blink-openapi-sdk/plugin"
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
								Value:    "",
								Usage:    "openApi file",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "name",
								Value:    "",
								Usage:    "pluginName",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "mask",
								Value:    "mask.yaml",
								Usage:    "maskFile",
								DefaultText: "mask.yaml",
							},
						},
						Name:    "Readme",
						Aliases: []string{"md"},
						Usage:   "Generate readme.md for openapi plugins",
						Action:  plugin.GenerateMarkdown,
					},
					{
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "file",
								Value:    "",
								Usage:    "openApi file",
								Required: true,
							},
						},
						Name:    "mask",
						Aliases: []string{"mask"},
						Usage:   "Generate mask.yaml for openapi plugins",
						Action:  plugin.GenerateMaskFile,
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
