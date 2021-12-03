package flickr_downloader

import "encoding/xml"

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
	XMLName   xml.Name `xml:"photo"`
	Id        string   `xml:"id,attr"`
	Owner     string   `xml:"owner,attr"`
	Title     string   `xml:"title,attr"`
	Secret    string   `xml:"secret,attr"`
	Server    string   `xml:"server,attr"`
	License   int
	PhotoInfo PhotoInfo
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
	FlickrUrl    string         `xml:"urls>url"`
}
