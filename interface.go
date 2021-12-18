package flickr_downloader

import (
	"github.com/gbdubs/attributions"
	"github.com/gbdubs/verbose"
)

type Input struct {
	FlickrAPIKey             string
	Query                    string
	OutputDir                string
	NumberOfImages           int
	ForceReload              bool
	IncludeAllRightsReserved bool
	verbose.Verbose
}

type Output struct {
	Files []attributions.AttributedFilePointer
}

func (i *Input) Execute() (*Output, error) {
	return i.execute()
}
