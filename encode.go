// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultEBPAJGuideVersion = "1.1.3"
	defaultKADOKAWAVersion   = "1.0"
)

// EncodeOption mutates encode behavior.
type EncodeOption func(*encodeConfig)

type encodeConfig struct {
	generateLegacyTOC bool
}

// WithLegacyTOC enables generation of EPUB 2 toc.ncx alongside EPUB 3 navigation.
func WithLegacyTOC() EncodeOption {
	return func(cfg *encodeConfig) {
		cfg.generateLegacyTOC = true
	}
}

// Encode writes a normalized Document into EPUB ZIP stream.
func Encode(w io.Writer, doc *Document, opts ...EncodeOption) error {
	if doc == nil {
		return fmt.Errorf("document is nil")
	}
	cfg := encodeConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	zw := zip.NewWriter(w)

	if err := writeMimetype(zw); err != nil {
		return err
	}
	if err := writeContainer(zw); err != nil {
		return err
	}
	if err := writePackageAndAssets(zw, doc, cfg); err != nil {
		return err
	}
	return zw.Close()
}

func writeMimetype(zw *zip.Writer) error {
	h := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, mimeTypeValue)
	return err
}

func writeContainer(zw *zip.Writer) error {
	w, err := zw.Create("META-INF/container.xml")
	if err != nil {
		return err
	}
	container := `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="item/standard.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
	_, err = io.WriteString(w, container)
	return err
}

func writePackageAndAssets(zw *zip.Writer, doc *Document, cfg encodeConfig) error {
	metadata := doc.effectiveMetadata()

	assetPaths := make([]string, 0, len(doc.Assets))
	for p := range doc.Assets {
		assetPaths = append(assetPaths, p)
	}
	sort.Strings(assetPaths)

	identifier := normalizeIdentifier(metadata.Identifier)
	identifierID := normalizeIdentifierID(metadata.IdentifierID)
	navHref := "nav.xhtml"

	coverAssetID := resolveCoverAssetID(doc, assetPaths)

	manifestItems := make([]string, 0, len(assetPaths))
	opfDir := "item"
	for _, p := range assetPaths {
		a := doc.Assets[p]
		href := p
		if rel, err := filepath.Rel(opfDir, p); err == nil {
			href = filepath.ToSlash(rel)
		}
		item := manifestItem{ID: a.ID, Href: href, MediaType: a.MimeType}
		if coverAssetID != "" && a.ID == coverAssetID {
			item.Properties = "cover-image"
		}
		b, err := xml.Marshal(item)
		if err != nil {
			return err
		}
		manifestItems = append(manifestItems, string(b))
	}
	navItemBytes, err := xml.Marshal(manifestItem{ID: "nav", Href: navHref, MediaType: "application/xhtml+xml", Properties: "nav"})
	if err != nil {
		return err
	}
	manifestItems = append(manifestItems, string(navItemBytes))
	if cfg.generateLegacyTOC {
		ncxItemBytes, err := xml.Marshal(manifestItem{ID: "ncx", Href: "toc.ncx", MediaType: "application/x-dtbncx+xml"})
		if err != nil {
			return err
		}
		manifestItems = append(manifestItems, string(ncxItemBytes))
	}

	spineItems := make([]string, 0, len(doc.Pages))
	for _, pg := range doc.Pages {
		prop := spreadToProperty(pg.Spread)
		if prop == "" {
			spineItems = append(spineItems, fmt.Sprintf(`<itemref idref=%q/>`, xmlEscape(pg.AssetID)))
			continue
		}
		spineItems = append(spineItems, fmt.Sprintf(`<itemref idref=%q properties=%q/>`, xmlEscape(pg.AssetID), xmlEscape(prop)))
	}

	rLayout := "reflowable"
	if doc.Layout == LayoutPrePaginated {
		rLayout = "pre-paginated"
	}
	spineTOC := ""
	if cfg.generateLegacyTOC {
		spineTOC = ` toc="ncx"`
	}

	metadataEntries := []string{
		fmt.Sprintf(`<dc:title id="title">%s</dc:title>`, xmlEscape(metadata.Title)),
		fmt.Sprintf(`<dc:identifier id=%q>%s</dc:identifier>`, xmlEscape(identifierID), xmlEscape(identifier)),
	}

	if v := strings.TrimSpace(metadata.TitleFileAs); v != "" {
		metadataEntries = append(metadataEntries, fmt.Sprintf(`<meta property="file-as" refines="#title">%s</meta>`, xmlEscape(v)))
	}
	for i, creator := range metadata.Creators {
		name := strings.TrimSpace(creator.Name)
		if name == "" {
			continue
		}
		creatorID := fmt.Sprintf("creator-%d", i+1)
		metadataEntries = append(metadataEntries, fmt.Sprintf(`<dc:creator id=%q>%s</dc:creator>`, xmlEscape(creatorID), xmlEscape(name)))
		if v := strings.TrimSpace(creator.FileAs); v != "" {
			metadataEntries = append(metadataEntries, fmt.Sprintf(`<meta property="file-as" refines=%q>%s</meta>`, xmlEscape("#"+creatorID), xmlEscape(v)))
		}
	}

	if coverAssetID != "" {
		coverMetaBytes, err := xml.Marshal(metadataMeta{Name: "cover", Content: coverAssetID})
		if err != nil {
			return err
		}
		metadataEntries = append(metadataEntries, string(coverMetaBytes))
	}

	metadataEntries = append(metadataEntries,
		fmt.Sprintf(`<meta property="rendition:layout">%s</meta>`, xmlEscape(rLayout)),
		fmt.Sprintf(`<meta property="dcterms:modified">%s</meta>`, xmlEscape(formatW3CTime(time.Now()))),
		fmt.Sprintf(`<meta property="ebpaj:guide-version">%s</meta>`, xmlEscape(normalizeSpecVersion(metadata.EBPAJGuideVersion, defaultEBPAJGuideVersion))),
		fmt.Sprintf(`<meta property="kadokawa:version">%s</meta>`, xmlEscape(normalizeSpecVersion(metadata.KADOKAWAVersion, defaultKADOKAWAVersion))),
	)

	opf := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf" unique-identifier=%q prefix="rendition: http://www.idpf.org/vocab/rendition/# dcterms: http://purl.org/dc/terms/ ebpaj: https://www.ebpaj.jp/ kadokawa: https://www.kadokawa.co.jp/">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    %s
  </metadata>
  <manifest>
    %s
  </manifest>
  <spine page-progression-direction=%q%s>
    %s
  </spine>
</package>`, xmlEscape(identifierID), strings.Join(metadataEntries, "\n    "), strings.Join(manifestItems, "\n    "), xmlEscape(normalizeDirection(doc.Direction)), spineTOC, strings.Join(spineItems, "\n    "))

	w, err := zw.Create("item/standard.opf")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, opf); err != nil {
		return err
	}

	nav, err := buildNavigationDocument(doc, navHref, coverAssetID)
	if err != nil {
		return err
	}
	navW, err := zw.Create(filepath.ToSlash(filepath.Join(opfDir, navHref)))
	if err != nil {
		return err
	}
	if _, err := io.WriteString(navW, nav); err != nil {
		return err
	}

	if cfg.generateLegacyTOC {
		ncx := buildLegacyNCX(doc, identifier)
		ncxW, err := zw.Create("item/toc.ncx")
		if err != nil {
			return err
		}
		if _, err := io.WriteString(ncxW, ncx); err != nil {
			return err
		}
	}

	for _, p := range assetPaths {
		a := doc.Assets[p]
		if a == nil || a.Open == nil {
			return fmt.Errorf("asset %s has no Open function", p)
		}
		aw, err := zw.Create(p)
		if err != nil {
			return err
		}
		rc, err := a.Open()
		if err != nil {
			return err
		}
		if _, err := io.Copy(aw, rc); err != nil {
			rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

func spreadToProperty(spread string) string {
	switch spread {
	case "left":
		return "page-spread-left"
	case "right":
		return "page-spread-right"
	case "center":
		return "page-spread-center"
	default:
		return ""
	}
}

func xmlEscape(v string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(v))
	return b.String()
}

func normalizeIdentifier(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "urn:uuid:generated"
	}
	return v
}

func normalizeIdentifierID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "pub-id"
	}
	return v
}

func normalizeSpecVersion(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func formatW3CTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func resolveCoverAssetID(doc *Document, sortedPaths []string) string {
	m := doc.effectiveMetadata()
	if id := strings.TrimSpace(m.CoverAssetID); id != "" {
		for _, a := range doc.Assets {
			if a != nil && a.ID == id {
				return id
			}
		}
		return ""
	}
	for _, p := range sortedPaths {
		a := doc.Assets[p]
		if a != nil && strings.HasPrefix(a.MimeType, "image/") {
			return a.ID
		}
	}
	return ""
}

func resolveCoverPageHref(doc *Document, coverAssetID string) string {
	if coverAssetID == "" {
		return ""
	}
	// Prefer page explicitly marked as cover.
	for _, pg := range doc.Pages {
		if pg.Type == PageTypeCover {
			href := strings.TrimSpace(pg.Href)
			if href != "" {
				return strings.TrimPrefix(href, "item/")
			}
		}
	}
	// Fallback: find XHTML wrapper matching coverAssetID.
	wrapperID := "xhtml-" + coverAssetID
	for _, pg := range doc.Pages {
		if pg.AssetID == wrapperID || pg.AssetID == coverAssetID {
			href := strings.TrimSpace(pg.Href)
			if href != "" {
				return strings.TrimPrefix(href, "item/")
			}
		}
	}
	return ""
}

func buildNavigationDocument(doc *Document, navHref string, coverAssetID string) (string, error) {
	tocItems := make([]navListItem, 0, len(doc.Pages))
	for i, pg := range doc.Pages {
		href := strings.TrimSpace(pg.Href)
		if href == "" {
			if a, err := doc.GetAssetByPage(pg); err == nil && a != nil {
				for p, asset := range doc.Assets {
					if asset == a {
						href = p
						break
					}
				}
			}
		}
		href = strings.TrimPrefix(href, "item/")
		if href == "" {
			href = "#"
		}
		tocItems = append(tocItems, navListItem{
			Anchor: navLandmarkAnchor{Href: href, Text: fmt.Sprintf("Page %d", i+1)},
		})
	}
	if len(tocItems) == 0 {
		tocItems = []navListItem{{Anchor: navLandmarkAnchor{Href: "#", Text: "Start"}}}
	}

	coverHref := resolveCoverPageHref(doc, coverAssetID)
	if coverHref == "" && len(doc.Pages) > 0 {
		first := strings.TrimSpace(doc.Pages[0].Href)
		if first != "" {
			coverHref = strings.TrimPrefix(first, "item/")
		}
	}
	if coverHref == "" {
		coverHref = "#"
	}

	bodyHref := "#"
	if len(doc.Pages) > 0 {
		first := strings.TrimSpace(doc.Pages[0].Href)
		if first != "" {
			bodyHref = strings.TrimPrefix(first, "item/")
		}
	}
	if bodyHref == "" {
		bodyHref = "#"
	}

	navDoc := navDocument{
		Head: navDocHead{Title: "Navigation"},
		Sections: []navSection{
			{
				EpubType:    "toc",
				ID:          "toc",
				HeadingTag:  "h1",
				HeadingText: "Navigation",
				Items:       tocItems,
			},
			{
				EpubType:    "landmarks",
				ID:          "landmarks",
				HeadingTag:  "h2",
				HeadingText: "Landmarks",
				Items: []navListItem{
					{Anchor: navLandmarkAnchor{EpubType: "cover", Href: coverHref, Text: "Cover"}},
					{Anchor: navLandmarkAnchor{EpubType: "toc", Href: navHref, Text: "Navigation"}},
					{Anchor: navLandmarkAnchor{EpubType: "bodymatter", Href: bodyHref, Text: "Body"}},
				},
			},
		},
	}

	b, err := xml.MarshalIndent(navDoc, "", "  ")
	if err != nil {
		return "", err
	}
	return xml.Header + string(b), nil
}

func buildLegacyNCX(doc *Document, identifier string) string {
	metadata := doc.effectiveMetadata()

	navPoints := make([]string, 0, len(doc.Pages))
	for i, pg := range doc.Pages {
		href := strings.TrimSpace(pg.Href)
		href = strings.TrimPrefix(cleanOPFRef("item", href), "item/")
		if href == "" {
			href = "#"
		}
		navPoints = append(navPoints, fmt.Sprintf(`<navPoint id="navPoint-%d" playOrder="%d"><navLabel><text>Page %d</text></navLabel><content src=%q/></navPoint>`, i+1, i+1, i+1, xmlEscape(href)))
	}
	if len(navPoints) == 0 {
		navPoints = append(navPoints, `<navPoint id="navPoint-1" playOrder="1"><navLabel><text>Start</text></navLabel><content src="#"/></navPoint>`)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head>
    <meta name="dtb:uid" content=%q/>
  </head>
  <docTitle><text>%s</text></docTitle>
  <navMap>
    %s
  </navMap>
</ncx>`, xmlEscape(identifier), xmlEscape(metadata.Title), strings.Join(navPoints, "\n    "))
}
