// SPDX-License-Identifier: Apache-2.0

package epub

import "encoding/xml"

type containerXML struct {
	XMLName   xml.Name           `xml:"container"`
	Rootfiles containerRootfiles `xml:"rootfiles"`
}

type containerRootfiles struct {
	Rootfile []containerRootfile `xml:"rootfile"`
}

type containerRootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

type packageXML struct {
	XMLName          xml.Name        `xml:"package"`
	UniqueIdentifier string          `xml:"unique-identifier,attr"`
	Metadata         packageMetadata `xml:"metadata"`
	Manifest         packageManifest `xml:"manifest"`
	Spine            packageSpine    `xml:"spine"`
}

type packageMetadata struct {
	Titles      []dcElement    `xml:"title"`
	Identifiers []dcElement    `xml:"identifier"`
	Creators    []dcElement    `xml:"creator"`
	Meta        []metadataMeta `xml:"meta"`
}

type dcElement struct {
	ID    string `xml:"id,attr"`
	Value string `xml:",chardata"`
}

type metadataMeta struct {
	Property string `xml:"property,attr"`
	Refines  string `xml:"refines,attr"`
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Value    string `xml:",chardata"`
}

type packageManifest struct {
	Items []manifestItem `xml:"item"`
}

type manifestItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type packageSpine struct {
	PageProgressionDirection string      `xml:"page-progression-direction,attr"`
	Itemrefs                 []spineItem `xml:"itemref"`
}

type spineItem struct {
	IDRef      string `xml:"idref,attr"`
	Properties string `xml:"properties,attr"`
}
