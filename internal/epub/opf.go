package epub

import (
	"encoding/xml"
	"path"
)

type Package struct {
	OpfPath  string   // the location of the OPF file
	XMLName  xml.Name `xml:"package"`
	Version  string   `xml:"version,attr"`
	Metadata Metadata `xml:"metadata"`
	Manifest Manifest `xml:"manifest"`
}

func (p *Package) ResolvePath(href string) string {
	opfDir := path.Dir(p.OpfPath)
	return path.Join(opfDir, href)
}

type Metadata struct {
	Title        []Title   `xml:"title"`
	Creators     []Creator `xml:"creator"`
	Descriptions []string  `xml:"description"`
	Languages    []string  `xml:"language"`
	Metas        []Meta    `xml:"meta"`
}

type Title struct {
	Value string `xml:",chardata"`
	ID    string `xml:"id,attr"`
	Lang  string `xml:"lang,attr"`
}

type Creator struct {
	Value  string `xml:",chardata"`    // the name of the creator (Rev. Dr. Martin Luther King Jr.)
	Role   string `xml:"role,attr"`    // 3 character long MARC value ("aut", "ill", etc)
	FileAs string `xml:"file-as,attr"` //  normalized form of the name (King, Martin Luther Jr.)
}

type Meta struct {
	Id       string `xml:"id,attr"`
	Refines  string `xml:"refines,attr"`
	Property string `xml:"property,attr"`
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Scheme   string `xml:"scheme,attr"`
	Value    string `xml:",chardata"`
}

type Item struct {
	ID         string `xml:"id,attr"`
	MediaType  string `xml:"media-type,attr"`
	Href       string `xml:"href,attr"`
	Properties string `xml:"properties,attr"`
}

type Manifest struct {
	Items []Item `xml:"item"`
}
