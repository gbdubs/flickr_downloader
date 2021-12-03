package flickr_downloader

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dsoprea/go-exif/v2"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure"
	"github.com/gbdubs/attributions"
)

func (input *Input) Execute() (*Output, error) {
	outputDir := input.OutputDir
	q := input.Query
	n := input.NumberOfImages
	a := input.FlickrAPIKey
	if outputDir == "" {
		outputDir = "/tmp/flickr_downloader"
	}
	folder := fmt.Sprintf("%s/%s", outputDir, q)
	os.MkdirAll(folder, 0777)
	photos, err := getFirstFlickrResultsWithSearchTerm(q, a, n)
	if err != nil {
		return &Output{}, err
	}
	errChans := make([]chan error, n, n)
	filePaths := make([]string, n, n)
	for i, photo := range photos {
		filePath := fmt.Sprintf("%s/%s_%d.jpeg", folder, q, i)
		filePaths[i] = filePath
		errChans[i] = photo.download(filePath, a)
	}
	for _, errChan := range errChans {
		err = <-errChan
		if err != nil {
			return &Output{}, err
		}
	}
	outputFiles := make([]OutputFile, n, n)
	for i, filePath := range filePaths {
		outputFiles[i] = OutputFile{
			OutputFilePath: filePath,
			Attribution:    *photos[i].attribution(),
		}
	}
	return &Output{
		OutputFiles: outputFiles,
	}, nil
}

// Licenses in order of least restrictive to most restrictive.
var licensesInPreferredOrder = [...]int{4, 5, 2, 1, 7, 6, 3, 9, 10, 8, 0}

func getFirstFlickrResultsWithSearchTerm(query string, apiKey string, n int) ([]*Photo, error) {
	// We use different batch sizes because of the unique authorship constraints.
	// These rough bounds were just chosen as a reasonable performance tradeoffs
	batchSize := 1
	if n > 1 {
		batchSize = 10
	}
	if n > 5 {
		batchSize = 25
	}
	result := make([]*Photo, n, n)
	uniqueOwners := make(map[string]bool)
	found := 0
	for _, license := range licensesInPreferredOrder {
		photos, err := searchPhotos(query, apiKey, license, batchSize)
		if err != nil {
			return result, err
		}
		for _, p := range photos.Photos {
			if !uniqueOwners[p.Owner] {
				uniqueOwners[p.Owner] = true
				result[found] = &p
				found++
				if found == n {
					return result, nil
				}
			}
		}
	}
	return result, nil
}

const API_ENDPOINT = "https://www.flickr.com/services/rest/"

func searchPhotos(query string, apiKey string, license int, numToList int) (Photos, error) {
	req, err := http.NewRequest("GET", API_ENDPOINT, nil)
	if err != nil {
		return Photos{}, err
	}
	q := req.URL.Query()
	q.Add("method", "flickr.photos.search")
	q.Add("api_key", apiKey)
	q.Add("license", strconv.Itoa(license))
	q.Add("per_page", strconv.Itoa(numToList))
	q.Add("text", query)
	q.Add("format", "rest")
	req.URL.RawQuery = q.Encode()
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Photos{}, err
	}
	defer resp.Body.Close()
	respAsBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Photos{}, err
	}
	var response Response
	err = xml.Unmarshal(respAsBytes, &response)
	if err != nil {
		return Photos{}, err
	}
	return response.Photos, nil
}

func (p *Photo) download(imagePath string, apiKey string) chan error {
	errChan := make(chan error)
	jpgChan := p.downloadJpg(imagePath)
	infoChan := p.downloadInfo(apiKey)
	go func() {
		if err := <-jpgChan; err != nil {
			errChan <- err
			return
		}
		if err := <-infoChan; err != nil {
			errChan <- err
			return
		}
		errChan <- p.setExifMetadata(imagePath)
	}()
	return errChan
}

func (p *Photo) downloadJpg(filePath string) chan error {
	errChan := make(chan error)
	go func() {
		url := fmt.Sprintf("https://live.staticflickr.com/%v/%v_%v_%v.jpg", p.Server, p.Id, p.Secret, "b")
		resp, err := http.Get(url)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()
		file, err := os.Create(filePath)
		if err != nil {
			errChan <- err
			return
		}
		defer file.Close()
		_, err = io.Copy(file, resp.Body)
		if err != nil {
			errChan <- err
			return
		}
		errChan <- nil
	}()
	return errChan
}

func (p *Photo) downloadInfo(apiKey string) chan error {
	errChan := make(chan error)
	go func() {
		req, err := http.NewRequest("GET", API_ENDPOINT, nil)
		if err != nil {
			errChan <- err
			return
		}
		q := req.URL.Query()
		q.Add("method", "flickr.photos.getinfo")
		q.Add("api_key", apiKey)
		q.Add("photo_id", p.Id)
		q.Add("format", "rest")
		req.URL.RawQuery = q.Encode()
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()
		respAsBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errChan <- err
			return
		}
		var response Response
		err = xml.Unmarshal(respAsBytes, &response)
		if err != nil {
			errChan <- err
			return
		}
		p.PhotoInfo = response.PhotoInfo
		errChan <- nil
	}()
	return errChan
}

func (p *Photo) attribution() *attributions.Attribution {
	i := p.PhotoInfo
	return &attributions.Attribution{
		OriginUrl:           i.FlickrUrl,
		CollectedAt:         time.Now(),
		OriginalTitle:       i.Title,
		Author:              fmt.Sprintf("%s (Flickr User %s)", i.Owner.RealName, i.Owner.UserName),
		AuthorUrl:           fmt.Sprintf("https://flickr.com/photos/%s", i.Owner.Id),
		License:             i.getLicenseName(),
		LicenseUrl:          i.getLicenseLink(),
		CreatedAt:           time.Unix(i.DateUploaded, 0),
		Context:             []string{i.Description},
		ScrapingMethodology: "github.com/gbdubs/flickr_downloader",
	}
}

func (p *Photo) setExifMetadata(jpegPath string) error {
	photo := p.PhotoInfo
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseFile(jpegPath)
	if err != nil {
		return err
	}
	sl := intfc.(*jpegstructure.SegmentList)
	rootIb, err := sl.ConstructExifBuilder()
	if err != nil {
		return err
	}
	// IFD0 Block
	ifd0Path := "IFD0"
	ifd0Ib, err := exif.GetOrCreateIbFromRootIb(rootIb, ifd0Path)
	if err != nil {
		return err
	}
	// Artist
	artist := fmt.Sprintf("%s (on flickr @%s)", photo.Owner.RealName, photo.Owner.UserName)
	err = ifd0Ib.SetStandardWithName("Artist", artist)
	if err != nil {
		return err
	}
	// Copyright
	copyright := photo.getLicenseDescription()
	err = ifd0Ib.SetStandardWithName("Copyright", copyright)
	if err != nil {
		return err
	}
	// Description
	description := fmt.Sprintf("%s\n%s\n%s", photo.Title, photo.Description, photo.FlickrUrl)
	err = ifd0Ib.SetStandardWithName("ImageDescription", description)
	if err != nil {
		return err
	}
	// DateTime - this is the time that the content was downloaded, not the time it was created.
	// this is largely to provide a point-in-time snapshot to point to if asked about when
	// content was scraped.
	dateTime := exif.ExifFullTimestampString(time.Unix(photo.DateUploaded, 0))
	err = ifd0Ib.SetStandardWithName("DateTime", dateTime)
	if err != nil {
		return err
	}
	err = sl.SetExif(rootIb)
	if err != nil {
		return err
	}
	b := bytes.NewBufferString("")
	err = sl.Write(b)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(jpegPath, b.Bytes(), 0666)
}

func (photo *PhotoInfo) getLicenseDescription() string {
	return fmt.Sprintf("%s (%s)", photo.getLicenseName(), photo.getLicenseLink())
}

func (photo *PhotoInfo) getLicenseName() string {
	switch photo.License {
	case 0:
		return "All Rights Reserved"
	case 1:
		return "Attribution-NonCommercial-ShareAlike License"
	case 2:
		return "Attribution-NonCommercial License"
	case 3:
		return "Attribution-NonCommercial-NoDerivs License"
	case 4:
		return "Attribution License"
	case 5:
		return "Attribution-ShareAlike License"
	case 6:
		return "Attribution-NoDerivs License"
	case 7:
		return "No known copyright restrictions"
	case 8:
		return "United States Government Work"
	case 9:
		return "Public Domain Dedication (CC0)"
	case 10:
		return "Public Domain Mark"
	}
	panic(errors.New(fmt.Sprintf("Unknown License Number: %d", photo.License)))
}

func (photo *PhotoInfo) getLicenseLink() string {
	switch photo.License {
	case 0:
		return ""
	case 1:
		return "https://creativecommons.org/licenses/by-nc-sa/2.0/"
	case 2:
		return "https://creativecommons.org/licenses/by-nc/2.0/"
	case 3:
		return "https://creativecommons.org/licenses/by-nc-nd/2.0/"
	case 4:
		return "https://creativecommons.org/licenses/by/2.0/"
	case 5:
		return "https://creativecommons.org/licenses/by-sa/2.0/"
	case 6:
		return "https://creativecommons.org/licenses/by-nd/2.0/"
	case 7:
		return "https://www.flickr.com/commons/usage/"
	case 8:
		return "http://www.usa.gov/copyright.shtml"
	case 9:
		return "https://creativecommons.org/publicdomain/zero/1.0/"
	case 10:
		return "https://creativecommons.org/publicdomain/mark/1.0/"
	}
	panic(errors.New(fmt.Sprintf("Unknown License Number: %d", photo.License)))
}
