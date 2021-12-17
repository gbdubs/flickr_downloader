package main

import (
	"errors"
	"log"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/gbdubs/flickr_downloader"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:    "Flickr Downloader",
		Usage:   "A CLI for downloading images from the image hosting app flickr that match a given query and have distinct authorship.",
		Version: "1.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "query",
				Aliases: []string{"q"},
				Usage:   "the term to search flickr for - can be one word or multiple words.",
			},
			&cli.StringFlag{
				Name:    "output_dir",
				Aliases: []string{"o"},
				Usage:   "where to place output, defaults to /tmp/flickr_downloader.",
			},
			&cli.IntFlag{
				Name:    "number_of_images",
				Aliases: []string{"n"},
				Usage:   "the number of distinct images to download, each with unique authorship.",
			},
			&cli.StringFlag{
				Name:    "api_key",
				Aliases: []string{"a"},
				Usage:   "The Flickr API Key to charge usage to.",
			},
			&cli.BoolFlag{
				Name:  "force_reload",
				Usage: "Whether to get additional images, even if the output directory already contains results.",
			},
			&cli.BoolFlag{
				Name:  "include_all_rights_reserved",
				Usage: "Whether to allow for license = 0 (NOT open access) outputs.",
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Whether to print the output or silently succeed, if the command succeeds.",
			},
		},
		Action: func(c *cli.Context) error {
			if c.String("query") == "" {
				return errors.New("query must be provided")
			}
			if c.String("api_key") == "" {
				return errors.New("api_key must be provided")
			}
			n := c.Int("number_of_images")
			if n <= 0 {
				n = 1
			}
			v := c.Bool("verbose")
			input := &flickr_downloader.Input{
				Query:                    c.String("query"),
				NumberOfImages:           n,
				OutputDir:                c.String("output_dir"),
				FlickrAPIKey:             c.String("api_key"),
				IncludeAllRightsReserved: c.Bool("include_all_rights_reserved"),
				ForceReload:              c.Bool("force_reload"),
				Verbose:                  v,
			}
			output, err := input.Execute()
			if err != nil {
				return err
			}
			if v {
				spew.Dump(*output)
			}
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
