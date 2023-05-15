package flickr_downloader

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gbdubs/attributions"
	"github.com/google/uuid"
)

func (input *Input) execute() (*Output, error) {
	o := &Output{}
	od := input.OutputDir
	if od == "" {
		od = fmt.Sprintf("/memo/flickr_downloader/%s", input.Query)
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

	photos, err := input.getFlickrSearchResults()
	if err != nil {
		return o, fmt.Errorf("Error calling flickr API with term %s: %v", input.Query, err)
	}
	n := len(photos)
	errChans := make([]chan error, 2*n)
	filePaths := make([]string, n)
	for i, photo := range photos {
		id := uuid.New().String()
		filePath := fmt.Sprintf("%s/%s.jpeg", od, id)
		filePaths[i] = filePath
		errChans[2*i] = photo.downloadJpg(filePath)
		errChans[2*i+1] = photo.downloadInfo(input.FlickrAPIKey)
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

func (input *Input) getFlickrSearchResults() ([]*Photo, error) {
	n := input.NumberOfImages
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
		if license == 0 && !input.IncludeAllRightsReserved {
			continue
		}
		foundInLastBatch := batchSize
		pageNumber := 1
		for foundInLastBatch == batchSize {
			ps, err := input.searchPhotos(license, batchSize, pageNumber)
			if len(ps.Photos) > 0 {
				input.VLog("found %d photos in page %d for query %s with license %d. ", len(ps.Photos), pageNumber, input.Query, license)
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
			if len(photos) > 0 {
				input.VLog("%d remaining. %d unique authors so far.\n", n-found, len(uniqueOwners))
			}
			pageNumber++
		}

	}
	return result[:found], nil
}

const API_ENDPOINT = "https://www.flickr.com/services/rest/"

func (input *Input) searchPhotos(license int, pageSize int, pageNumber int) (Photos, error) {
	req, err := http.NewRequest("GET", API_ENDPOINT, nil)
	if err != nil {
		return Photos{}, err
	}
	q := req.URL.Query()
	q.Add("method", "flickr.photos.search")
	q.Add("api_key", input.FlickrAPIKey)
	q.Add("license", strconv.Itoa(license))
	q.Add("per_page", strconv.Itoa(pageSize))
	q.Add("page", strconv.Itoa(pageNumber))
	q.Add("text", input.Query)
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
		fmt.Printf("\n\n%s\n\n", string(respAsBytes))
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
			fmt.Printf("RESPONSE\n\n%s\n\n", string(respAsBytes))
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
