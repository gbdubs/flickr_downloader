package flickr_downloader

import "github.com/gbdubs/attributions"

type Input struct {
	FlickrAPIKey             string
	Query                    string
	OutputDir                string
	NumberOfImages           int
	ForceReload              bool
	IncludeAllRightsReserved bool
	Verbose                  bool
}

type Output struct {
	Files []attributions.AttributedFilePointer
}
