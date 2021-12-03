package flickr_downloader

import "github.com/gbdubs/attributions"

type Input struct {
	FlickrAPIKey   string
	Query          string
	NumberOfImages int
	OutputDir      string
}

type OutputFile struct {
	OutputFilePath string
	Attribution    attributions.Attribution
}

type Output struct {
	OutputFiles []OutputFile
}
