package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/dsoprea/go-exif/v2"
	"github.com/dsoprea/go-jpeg-image-structure"
	"github.com/urfave/cli/v2"
	"io"
	"io/ioutil"
	"net/http"
	"os"
  "log"
	"strconv"
	"time"
)

type Parameters struct {
	Query          string
	NumberOfImages int
	OutputLocation string
}

func main() {
  if API_KEY == DEFAULT_API_KEY {
    fmt.Println("In order to use this script, you'll need an API key from Flickr. It's easy, just follow these steps:\n1) Request an appropriate API key here: https://www.flickr.com/services/apps/create/apply/\n2) Once you have your key, modify this file where it says `const API_KEY = \"PutYourAPIKeyHere!\"` and replace that with your API Key.\n3) Run `go build` on the command line.\n4)Run flickr_downloader --help to see your options!\nThanks for using this, and let me know if you have any questions!")
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
				Name:    "output_path",
				Aliases: []string{"o"},
				Usage:   "where to place output, defaults to current directory.",
			},
			&cli.IntFlag{
				Name:    "number_of_images",
				Aliases: []string{"n"},
				Usage:   "the number of distinct images to download, each with unique authorship.",
			},
		},
		Action: func(c *cli.Context) error {
			if c.String("query") == "" {
				return errors.New("Query must be provided")
			}
			n := c.Int("number_of_images")
			if n <= 0 {
				n = 1
			}
			o := c.String("output_path")
			if o == "" {
				p, err := os.Getwd()
				if err != nil {
					return err
				}
				o = p
			}
			return Execute(Parameters{
				Query:          c.String("query"),
				NumberOfImages: n,
				OutputLocation: o,
			})
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func Execute(p Parameters) error {
	folder := fmt.Sprintf("%s/%s", p.OutputLocation, p.Query)
	os.MkdirAll(folder, os.ModePerm)
  q := p.Query
  n := p.NumberOfImages
  photos, err := getFirstFlickrResultsWithSearchTerm(q, n); if err != nil {
		return err
	}
	errChan := make(chan error)
	for i, photo := range photos {
		go photo.download(fmt.Sprintf("%s/%s_%d", folder, q, i), errChan)
	}
	for _, _ = range photos {
		err = <-errChan; if err != nil {
			return err
		}
	}
	return nil
}

func getFirstFlickrResultsWithSearchTerm(query string, n int) ([]Photo, error) {
  // Licenses in order of least restrictive to most restrictive. 
	licensesInPreferredOrder := [...]int{4, 5, 2, 1, 7, 6, 3, 9, 10, 8, 0}
  // We use different batch sizes because of the unique authorship constraints.
  // These rough bounds were just chosen as a reasonable performance tradeoffs
  batchSize := 1
  if n > 1 {
    batchSize = 10
  }
  if n > 5 {
    batchSize = 25
  }
	var result []Photo
	for _, license := range licensesInPreferredOrder {
		photos, err := searchPhotos(query, license, batchSize);	if err != nil {
			return result, err
		}
		for _, p := range photos.Photos {
			if photoHasUniqueOwner(result, p) {
				result = append(result, p)
			}
		}
		if len(result) >= n {
			return result[0:n], nil
		}
	}
	return result, nil
}

func photoHasUniqueOwner(ps []Photo, o Photo) bool {
	for _, p := range ps {
		if p.Owner == o.Owner {
			return false
		}
	}
	return true
}

type Response struct {
	XMLName   xml.Name  `xml:"rsp"`
	Photos    Photos    `xml:"photos"`
	PhotoInfo PhotoInfo `xml:"photo"`
}

type Photos struct {
	XMLName xml.Name `xml:"photos"`
	Page    int32    `xml:"page,attr"`
	Pages   int32    `xml:"pages,attr"`
	Total   int32    `xml:"total,attr"`
	Photos  []Photo  `xml:"photo"`
}

type Photo struct {
	XMLName xml.Name `xml:"photo"`
	Id      string   `xml:"id,attr"`
	Owner   string   `xml:"owner,attr"`
	Title   string   `xml:"title,attr"`
	Secret  string   `xml:"secret,attr"`
	Server  string   `xml:"server,attr"`
	License int
}

type PhotoInfoOwner struct {
	XMLName  xml.Name `xml:"owner"`
	Id       string   `xml:"nsid,attr"`
	UserName string   `xml:"username,attr"`
	RealName string   `xml:"realname,attr"`
}

type PhotoInfo struct {
	XMLName      xml.Name       `xml:"photo"`
	DateUploaded int64          `xml:"dateuploaded,attr"`
	License      int            `xml:"license,attr"`
	Owner        PhotoInfoOwner `xml:"owner"`
	Title        string         `xml:"title"`
	Description  string         `xml:"description"`
	FlickrUrl     string         `xml:"urls>url"`
}

const API_ENDPOINT = "https://www.flickr.com/services/rest/"
const API_KEY = "PutYourAPIKeyHere"
const DEFUAULT_API_KEY = "PutYourAPIKeyHere"

func searchPhotos(query string, license int, numToList int) (Photos, error) {
	req, err := http.NewRequest("GET", API_ENDPOINT, nil)
	if err != nil {
		return Photos{}, err
	}
	q := req.URL.Query()
	q.Add("method", "flickr.photos.search")
	q.Add("api_key", API_KEY)
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

func (p Photo) download(filePath string, cOuter chan error) {
	imagePath := filePath + ".jpeg"
	c := make(chan error)
	i := make(chan PhotoInfo)
	go p.downloadJpg(imagePath, c)
	go p.downloadInfo(c, i)
	err := <-c
	if err != nil {
		cOuter <- err
		return
	}
	err = <-c
	if err != nil {
		cOuter <- err
		return
	}
	photoInfo := <-i
	err = setExifMetadata(imagePath, photoInfo)
	if err != nil {
		cOuter <- err
		return
	}
	cOuter <- nil
}

func (p Photo) downloadJpg(filePath string, errChan chan error) {
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
}

func (p Photo) downloadInfo(errChan chan error, resChan chan PhotoInfo) {
	req, err := http.NewRequest("GET", API_ENDPOINT, nil)
	if err != nil {
		errChan <- err
		return
	}
	q := req.URL.Query()
	q.Add("method", "flickr.photos.getinfo")
	q.Add("api_key", API_KEY)
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
	errChan <- nil
	resChan <- response.PhotoInfo
}

func setExifMetadata(jpegPath string, photo PhotoInfo) error {
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
	copyright := photo.getCopyrightString()
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

func (photo PhotoInfo) getCopyrightString() string {
	switch photo.License {
	case 0:
		return "All Rights Reserved"
	case 1:
		return "Attribution-NonCommercial-ShareAlike License (https://creativecommons.org/licenses/by-nc-sa/2.0/)"
	case 2:
		return "Attribution-NonCommercial License (https://creativecommons.org/licenses/by-nc/2.0/)"
	case 3:
		return "Attribution-NonCommercial-NoDerivs License (https://creativecommons.org/licenses/by-nc-nd/2.0/)"
	case 4:
		return "Attribution License (https://creativecommons.org/licenses/by/2.0/)"
	case 5:
		return "Attribution-ShareAlike License (https://creativecommons.org/licenses/by-sa/2.0/)"
	case 6:
		return "Attribution-NoDerivs License (https://creativecommons.org/licenses/by-nd/2.0/)"
	case 7:
		return "No known copyright restrictions (https://www.flickr.com/commons/usage/)"
	case 8:
		return "United States Government Work (http://www.usa.gov/copyright.shtml)"
	case 9:
		return "Public Domain Dedication (CC0) (https://creativecommons.org/publicdomain/zero/1.0/)"
	case 10:
		return "Public Domain Mark (https://creativecommons.org/publicdomain/mark/1.0/)"
	}
	panic(errors.New(fmt.Sprintf("Unknown License Number: %d", photo.License)))
}
