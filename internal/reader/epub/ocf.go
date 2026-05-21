package epub

import "encoding/xml"

type Container struct {
	XMLName   xml.Name  `xml:"container"`
	Version   string    `xml:"version,attr"`
	Rootfiles Rootfiles `xml:"rootfiles"`
}

type Rootfiles struct {
	RootfileList []Rootfile `xml:"rootfile"`
}

type Rootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}
