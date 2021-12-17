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
	"github.com/google/uuid"
)

func (input *Input) execute() (*Output, error) {
	o := &Output{}
	q := input.Query
	n := input.NumberOfImages
	a := input.FlickrAPIKey
	v := input.Verbose
	izl := input.IncludeAllRightsReserved
	od := input.OutputDir
	if od == "" {
		od = fmt.Sprintf("/memo/flickr_downloader/%s", q)
	}
	err := os.MkdirAll(od, 0777)

	if err != nil {
		return o, fmt.Errorf("Error creating output directory at %s: %v", od, err)
	}
	alreadyExisting, err := attributions.ReadAllAttributedFilePointers(od)
	if err != nil {
		return o, fmt.Errorf("Error reading cached images at %s: %v", od, err)
	}
	if len(alreadyExisting) > 0 && !input.ForceReload {
		o.Files = alreadyExisting
		return o, nil
	}

	photos, err := getFirstFlickrResultsWithSearchTerm(q, a, n, izl, v)
	if err != nil {
		return o, fmt.Errorf("Error calling flickr API with term %s: %v", q, err)
	}
	n = len(photos)
	errChans := make([]chan error, 2*n)
	filePaths := make([]string, n)
	for i, photo := range photos {
		id := uuid.New().String()
		filePath := fmt.Sprintf("%s/%s.jpeg", od, id)
		filePaths[i] = filePath
		errChans[2*i] = photo.downloadJpg(filePath)
		errChans[2*i+1] = photo.downloadInfo(a)
	}
	for _, errChan := range errChans {
		err = <-errChan
		if err != nil {
			return o, fmt.Errorf("Error with secondary flickr api calls (info/jpg download): %v", err)
		}
	}
	outputFiles := make([]attributions.AttributedFilePointer, 0)
	for i, filePath := range filePaths {
		afp, err := attributions.AttributeLocalFile(filePath, *photos[i].attribution())
		if err != nil {
			return o, fmt.Errorf("Error attributing file at %s: %v", filePath, err)
		}
		outputFiles = append(outputFiles, afp)
	}
	o.Files = outputFiles
	return o, nil
}

// Licenses in order of least restrictive to most restrictive.
var licensesInPreferredOrder = [...]int{4, 5, 2, 1, 7, 6, 3, 9, 10, 8, 0}

func getFirstFlickrResultsWithSearchTerm(query string, apiKey string, n int, includeZeroLicense bool, verbose bool) ([]*Photo, error) {
	// We use different batch sizes because of the unique authorship constraints.
	// These rough bounds were just chosen as a reasonable performance tradeoffs
	batchSize := 1
	if n > 1 {
		batchSize = 100
	}
	if n > 5 {
		batchSize = 500
	}
	result := make([]*Photo, n, n)
	uniqueOwners := make(map[string]bool)
	found := 0
	for _, license := range licensesInPreferredOrder {
		if license == 0 && !includeZeroLicense {
			continue
		}
		foundInLastBatch := batchSize
		pageNumber := 1
		for foundInLastBatch == batchSize {
			ps, err := searchPhotos(query, apiKey, license, batchSize, pageNumber)
			if verbose {
				fmt.Printf("  found %d photos in page %d for query %s with license %d. ", len(ps.Photos), pageNumber, query, license)
			}
			if err != nil {
				return result, err
			}
			photos := make([]*Photo, len(ps.Photos))
			foundInLastBatch = len(ps.Photos)
			for i, _ := range ps.Photos {
				// OOOH TRICKSY POINTERSES WE HATES THEM WE DO
				photos[i] = &ps.Photos[i]
			}
			for _, p := range photos {
				owner := p.Ownername
				if !uniqueOwners[owner] {
					uniqueOwners[owner] = true
					result[found] = p
					found++
					if found == n {
						return result, nil
					}
				}
			}
			if verbose {
				fmt.Printf("%d remaining. %d unique authors so far.\n", n-found, len(uniqueOwners))
			}
			pageNumber++
		}

	}
	return result[:found], nil
}

const API_ENDPOINT = "https://www.flickr.com/services/rest/"

func searchPhotos(query string, apiKey string, license int, pageSize int, pageNumber int) (Photos, error) {
	req, err := http.NewRequest("GET", API_ENDPOINT, nil)
	if err != nil {
		return Photos{}, err
	}
	q := req.URL.Query()
	q.Add("method", "flickr.photos.search")
	q.Add("api_key", apiKey)
	q.Add("license", strconv.Itoa(license))
	q.Add("per_page", strconv.Itoa(pageSize))
	q.Add("page", strconv.Itoa(pageNumber))
	q.Add("text", query)
	q.Add("media", "photos")
	q.Add("sort", "relevance")
	q.Add("format", "rest")
	q.Add("extras", "owner_name")
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
