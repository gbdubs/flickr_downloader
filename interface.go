package flickr_downloader

import "github.com/gbdubs/attributions"

type Input struct {
	FlickrAPIKey             string
	Query                    string
	NumberOfImages           int
	IncludeAllRightsReserved bool
	OutputDir                string
	Verbose                  bool
}

type OutputFile struct {
	OutputFilePath string
	Attribution    attributions.Attribution
}

type Output struct {
	OutputFiles []OutputFile
}
