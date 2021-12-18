package flickr_downloader

import (
	"errors"
	"fmt"
)

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
