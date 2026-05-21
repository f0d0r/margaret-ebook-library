package epub

import "encoding/xml"

type Package struct {
	XMLName  xml.Name `xml:"package"`
	Version  string   `xml:"version,attr"`
	Metadata Metadata `xml:"metadata"`
}

type Metadata struct {
	Title []Title `xml:"title"`
}

type Title struct {
	Value string `xml:",chardata"`
	ID    string `xml:"id,attr"`
	Lang  string `xml:"lang,attr"`
}
