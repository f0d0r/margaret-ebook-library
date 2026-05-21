package epub

import "encoding/xml"

type Package struct {
	XMLName  xml.Name `xml:"package"`
	Version  string   `xml:"version,attr"`
	Metadata Metadata `xml:"metadata"`
}

type Metadata struct {
	Title        []Title   `xml:"title"`
	Creators     []Creator `xml:"creator"`
	Descriptions []string  `xml:"description"`
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
