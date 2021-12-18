package flickr_downloader

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/dsoprea/go-exif/v2"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure"
)

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
	// content was scraped. Critically we wouldn't want modifications in download time
	// yield different SHA256s.
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
