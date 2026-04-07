// SPDX-License-Identifier: Apache-2.0

package epub

import "encoding/xml"

type containerXML struct {
	XMLName   xml.Name           `xml:"container"`
	Rootfiles containerRootfiles `xml:"rootfiles"`
}

func (c containerXML) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "container"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "version"}, Value: "1.0"},
		{Name: xml.Name{Local: "xmlns"}, Value: "urn:oasis:names:tc:opendocument:xmlns:container"},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeElement(c.Rootfiles, xml.StartElement{Name: xml.Name{Local: "rootfiles"}}); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
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
	Version          string          `xml:"version,attr,omitempty"`
	UniqueIdentifier string          `xml:"unique-identifier,attr"`
	Prefix           string          `xml:"prefix,attr,omitempty"`
	Metadata         packageMetadata `xml:"metadata"`
	Manifest         packageManifest `xml:"manifest"`
	Spine            packageSpine    `xml:"spine"`
}

func (p packageXML) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "package"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "version"}, Value: p.Version},
		{Name: xml.Name{Local: "xmlns"}, Value: "http://www.idpf.org/2007/opf"},
		{Name: xml.Name{Local: "unique-identifier"}, Value: p.UniqueIdentifier},
		{Name: xml.Name{Local: "prefix"}, Value: p.Prefix},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeElement(p.Metadata, xml.StartElement{Name: xml.Name{Local: "metadata"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(p.Manifest, xml.StartElement{Name: xml.Name{Local: "manifest"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(p.Spine, xml.StartElement{Name: xml.Name{Local: "spine"}}); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

type packageMetadata struct {
	Titles      []dcElement    `xml:"title"`
	Identifiers []dcElement    `xml:"identifier"`
	Creators    []dcElement    `xml:"creator"`
	Meta        []metadataMeta `xml:"meta"`
}

func (m packageMetadata) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "metadata"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "xmlns:dc"}, Value: "http://purl.org/dc/elements/1.1/"},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, t := range m.Titles {
		if err := encodeDCElement(e, "dc:title", t); err != nil {
			return err
		}
	}
	for _, id := range m.Identifiers {
		if err := encodeDCElement(e, "dc:identifier", id); err != nil {
			return err
		}
	}
	for _, c := range m.Creators {
		if err := encodeDCElement(e, "dc:creator", c); err != nil {
			return err
		}
	}
	for _, meta := range m.Meta {
		if err := e.EncodeElement(meta, xml.StartElement{Name: xml.Name{Local: "meta"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

func encodeDCElement(e *xml.Encoder, name string, dc dcElement) error {
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if dc.ID != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: dc.ID})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.CharData(dc.Value)); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

type dcElement struct {
	ID    string `xml:"id,attr"`
	Value string `xml:",chardata"`
}

type metadataMeta struct {
	XMLName  xml.Name `xml:"meta"`
	Property string   `xml:"property,attr,omitempty"`
	Refines  string   `xml:"refines,attr,omitempty"`
	Name     string   `xml:"name,attr,omitempty"`
	Content  string   `xml:"content,attr,omitempty"`
	Value    string   `xml:",chardata"`
}

type packageManifest struct {
	Items []manifestItem `xml:"item"`
}

type manifestItem struct {
	XMLName    xml.Name `xml:"item"`
	ID         string   `xml:"id,attr"`
	Href       string   `xml:"href,attr"`
	MediaType  string   `xml:"media-type,attr"`
	Properties string   `xml:"properties,attr,omitempty"`
}

type packageSpine struct {
	PageProgressionDirection string      `xml:"page-progression-direction,attr"`
	TOC                      string      `xml:"toc,attr,omitempty"`
	Itemrefs                 []spineItem `xml:"itemref"`
}

func (s packageSpine) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "spine"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "page-progression-direction"}, Value: s.PageProgressionDirection},
	}
	if s.TOC != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "toc"}, Value: s.TOC})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, item := range s.Itemrefs {
		if err := e.EncodeElement(item, xml.StartElement{Name: xml.Name{Local: "itemref"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

type spineItem struct {
	IDRef      string `xml:"idref,attr"`
	Properties string `xml:"properties,attr,omitempty"`
}

type xhtmlMeta struct {
	XMLName xml.Name `xml:"meta"`
	Name    string   `xml:"name,attr"`
	Content string   `xml:"content,attr"`
}

type xhtmlImg struct {
	XMLName xml.Name `xml:"img"`
	Src     string   `xml:"src,attr"`
	Alt     string   `xml:"alt,attr"`
}

type xhtmlHead struct {
	Title string    `xml:"title"`
	Meta  xhtmlMeta `xml:"meta"`
	Style string    `xml:"style"`
}

type xhtmlBody struct {
	Img xhtmlImg `xml:"img"`
}

type xhtmlDocument struct {
	XMLName   xml.Name  `xml:"html"`
	XMLNS     string    `xml:"xmlns,attr"`
	XMLNSEpub string    `xml:"xmlns:epub,attr"`
	Head      xhtmlHead `xml:"head"`
	Body      xhtmlBody `xml:"body"`
}

// navLandmarkAnchor represents an <a> element in the navigation landmarks.
// It uses a custom MarshalXML to emit the epub:type attribute with a prefixed
// name (not a namespace URI) so the output is compatible with the parent
// <html xmlns:epub="…"> declaration.
type navLandmarkAnchor struct {
	EpubType string
	Href     string
	Text     string
}

func (a navLandmarkAnchor) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "a"}
	if a.EpubType != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: "epub:type"},
			Value: a.EpubType,
		})
	}
	start.Attr = append(start.Attr, xml.Attr{
		Name:  xml.Name{Local: "href"},
		Value: a.Href,
	})
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.CharData(a.Text)); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// navDocument represents the full navigation XHTML document (nav.xhtml).
type navDocument struct {
	Head     navDocHead
	Sections []navSection
}

func (d navDocument) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "html"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/1999/xhtml"},
		{Name: xml.Name{Local: "xmlns:epub"}, Value: "http://www.idpf.org/2007/ops"},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeElement(d.Head, xml.StartElement{Name: xml.Name{Local: "head"}}); err != nil {
		return err
	}
	bodyStart := xml.StartElement{Name: xml.Name{Local: "body"}}
	if err := e.EncodeToken(bodyStart); err != nil {
		return err
	}
	for i := range d.Sections {
		if err := e.EncodeElement(d.Sections[i], xml.StartElement{Name: xml.Name{Local: "nav"}}); err != nil {
			return err
		}
	}
	if err := e.EncodeToken(bodyStart.End()); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

type navDocHead struct {
	Title string `xml:"title"`
}

// navSection represents a <nav> element with an epub:type attribute.
type navSection struct {
	EpubType    string
	ID          string
	HeadingTag  string // e.g. "h1", "h2"
	HeadingText string
	Items       []navListItem
}

func (s navSection) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "nav"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "epub:type"}, Value: s.EpubType},
		{Name: xml.Name{Local: "id"}, Value: s.ID},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	hStart := xml.StartElement{Name: xml.Name{Local: s.HeadingTag}}
	if err := e.EncodeToken(hStart); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.CharData(s.HeadingText)); err != nil {
		return err
	}
	if err := e.EncodeToken(hStart.End()); err != nil {
		return err
	}
	olStart := xml.StartElement{Name: xml.Name{Local: "ol"}}
	if err := e.EncodeToken(olStart); err != nil {
		return err
	}
	for i := range s.Items {
		if err := e.EncodeElement(s.Items[i], xml.StartElement{Name: xml.Name{Local: "li"}}); err != nil {
			return err
		}
	}
	if err := e.EncodeToken(olStart.End()); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// navListItem represents a <li> element wrapping an anchor.
type navListItem struct {
	Anchor navLandmarkAnchor
}

func (li navListItem) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "li"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeElement(li.Anchor, xml.StartElement{Name: xml.Name{Local: "a"}}); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// NCX types for EPUB 2 toc.ncx generation.

type ncxDocument struct {
	Head     ncxHead
	DocTitle ncxDocTitle
	NavMap   ncxNavMap
}

func (n ncxDocument) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "ncx"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "xmlns"}, Value: "http://www.daisy.org/z3986/2005/ncx/"},
		{Name: xml.Name{Local: "version"}, Value: "2005-1"},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeElement(n.Head, xml.StartElement{Name: xml.Name{Local: "head"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(n.DocTitle, xml.StartElement{Name: xml.Name{Local: "docTitle"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(n.NavMap, xml.StartElement{Name: xml.Name{Local: "navMap"}}); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

type ncxHead struct {
	Metas []ncxMeta `xml:"meta"`
}

type ncxMeta struct {
	XMLName xml.Name `xml:"meta"`
	Name    string   `xml:"name,attr"`
	Content string   `xml:"content,attr"`
}

type ncxDocTitle struct {
	Text string `xml:"text"`
}

type ncxNavMap struct {
	NavPoints []ncxNavPoint `xml:"navPoint"`
}

type ncxNavPoint struct {
	XMLName   xml.Name    `xml:"navPoint"`
	ID        string      `xml:"id,attr"`
	PlayOrder int         `xml:"playOrder,attr"`
	NavLabel  ncxNavLabel `xml:"navLabel"`
	Content   ncxContent  `xml:"content"`
}

type ncxNavLabel struct {
	Text string `xml:"text"`
}

type ncxContent struct {
	XMLName xml.Name `xml:"content"`
	Src     string   `xml:"src,attr"`
}
