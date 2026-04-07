// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
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
	generateLegacyTOC   bool
	preflightCompliance ComplianceLevel
	warningCollector    func(string)
}

// WithLegacyTOC enables generation of EPUB 2 toc.ncx alongside EPUB 3 navigation.
func WithLegacyTOC() EncodeOption {
	return func(cfg *encodeConfig) {
		cfg.generateLegacyTOC = true
	}
}

// WithEncodePreflightCompliance enables preflight compliance checks before
// writing the EPUB ZIP.  Only strict profiles (LevelEBPAJ, LevelKADOKAWA) run
// actual checks; LevelFlexible is accepted but performs no validation.
// Detected violations are delivered as non-fatal warnings through the collector
// registered via [WithEncodeWarningCollector].
func WithEncodePreflightCompliance(level ComplianceLevel) EncodeOption {
	return func(cfg *encodeConfig) {
		cfg.preflightCompliance = level
	}
}

// WithEncodeWarningCollector registers a callback that receives each preflight
// warning string.  The callback is invoked synchronously before ZIP writing
// begins.  If no collector is set, preflight warnings are silently discarded.
func WithEncodeWarningCollector(fn func(string)) EncodeOption {
	return func(cfg *encodeConfig) {
		cfg.warningCollector = fn
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

	if cfg.preflightCompliance != LevelFlexible {
		warnings := preflightEncode(cfg.preflightCompliance, doc)
		if cfg.warningCollector != nil {
			for _, w := range warnings {
				cfg.warningCollector(w)
			}
		}
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
	c := containerXML{
		Rootfiles: containerRootfiles{
			Rootfile: []containerRootfile{
				{FullPath: "item/standard.opf", MediaType: "application/oebps-package+xml"},
			},
		},
	}
	b, err := xml.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, xml.Header+string(b))
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

	opfDir := "item"
	items := make([]manifestItem, 0, len(assetPaths)+2)
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
		items = append(items, item)
	}
	items = append(items, manifestItem{ID: "nav", Href: navHref, MediaType: "application/xhtml+xml", Properties: "nav"})
	if cfg.generateLegacyTOC {
		items = append(items, manifestItem{ID: "ncx", Href: "toc.ncx", MediaType: "application/x-dtbncx+xml"})
	}

	spineItems := make([]spineItem, 0, len(doc.Pages))
	for _, pg := range doc.Pages {
		spineItems = append(spineItems, spineItem{
			IDRef:      pg.AssetID,
			Properties: spreadToProperty(pg.Spread),
		})
	}

	rLayout := "reflowable"
	if doc.Layout == LayoutPrePaginated {
		rLayout = "pre-paginated"
	}

	titles := []dcElement{{ID: "title", Value: metadata.Title}}
	identifiers := []dcElement{{ID: identifierID, Value: identifier}}
	languages := []dcElement{{Value: metadata.Language}}

	var creators []dcElement
	var metaEntries []metadataMeta

	if v := strings.TrimSpace(metadata.TitleFileAs); v != "" {
		metaEntries = append(metaEntries, metadataMeta{Property: "file-as", Refines: "#title", Value: v})
	}
	for i, creator := range metadata.Creators {
		name := strings.TrimSpace(creator.Name)
		if name == "" {
			continue
		}
		creatorID := fmt.Sprintf("creator-%d", i+1)
		creators = append(creators, dcElement{ID: creatorID, Value: name})
		if v := strings.TrimSpace(creator.FileAs); v != "" {
			metaEntries = append(metaEntries, metadataMeta{Property: "file-as", Refines: "#" + creatorID, Value: v})
		}
	}

	if coverAssetID != "" {
		metaEntries = append(metaEntries, metadataMeta{Name: "cover", Content: coverAssetID})
	}

	rSpread := normalizeRenditionSpread(metadata.RenditionSpread)
	rOrientation := normalizeRenditionOrientation(metadata.RenditionOrientation)

	metaEntries = append(metaEntries,
		metadataMeta{Property: "rendition:layout", Value: rLayout},
		metadataMeta{Property: "rendition:spread", Value: rSpread},
		metadataMeta{Property: "rendition:orientation", Value: rOrientation},
		metadataMeta{Property: "dcterms:modified", Value: formatW3CTime(time.Now())},
		metadataMeta{Property: "ebpaj:guide-version", Value: normalizeSpecVersion(metadata.EBPAJGuideVersion, defaultEBPAJGuideVersion)},
		metadataMeta{Property: "kadokawa:version", Value: normalizeSpecVersion(metadata.KADOKAWAVersion, defaultKADOKAWAVersion)},
	)

	// For pre-paginated layouts, auto-inject Kindle compatibility metadata.
	if doc.Layout == LayoutPrePaginated && len(doc.Pages) > 0 {
		var basePage *Page

		// Find the first non-cover page to use as the baseline resolution.
		for _, p := range doc.Pages {
			if p.Type != PageTypeCover && p.Width > 0 && p.Height > 0 {
				basePage = p
				break
			}
		}

		// Fallback to the first page if all pages are covers or lack dimensions.
		if basePage == nil && doc.Pages[0].Width > 0 && doc.Pages[0].Height > 0 {
			basePage = doc.Pages[0]
		}

		if basePage != nil {
			res := fmt.Sprintf("%dx%d", basePage.Width, basePage.Height)
			metaEntries = append(metaEntries,
				metadataMeta{Name: "original-resolution", Content: res},
			)
		}

		bookType := strings.TrimSpace(metadata.BookType)
		if bookType != "" {
			metaEntries = append(metaEntries,
				metadataMeta{Name: "book-type", Content: bookType},
			)
		}
	}

	spineTOC := ""
	if cfg.generateLegacyTOC {
		spineTOC = "ncx"
	}

	pkg := packageXML{
		Version:          "3.0",
		UniqueIdentifier: identifierID,
		Prefix:           "rendition: http://www.idpf.org/vocab/rendition/# dcterms: http://purl.org/dc/terms/ ebpaj: https://www.ebpaj.jp/ kadokawa: https://www.kadokawa.co.jp/",
		Metadata: packageMetadata{
			Titles:      titles,
			Identifiers: identifiers,
			Languages:   languages,
			Creators:    creators,
			Meta:        metaEntries,
		},
		Manifest: packageManifest{Items: items},
		Spine: packageSpine{
			PageProgressionDirection: normalizeDirection(doc.Direction),
			TOC:                      spineTOC,
			Itemrefs:                 spineItems,
		},
	}

	b, err := xml.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}

	w, err := zw.Create("item/standard.opf")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, xml.Header+string(b)); err != nil {
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
		ncx, err := buildLegacyNCX(doc, identifier)
		if err != nil {
			return err
		}
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
			_ = rc.Close()
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

func normalizeRenditionSpread(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "none", "landscape", "portrait", "both", "auto":
		return v
	default:
		return "landscape"
	}
}

func normalizeRenditionOrientation(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "landscape", "portrait", "auto":
		return v
	default:
		return "auto"
	}
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

func buildLegacyNCX(doc *Document, identifier string) (string, error) {
	metadata := doc.effectiveMetadata()

	navPoints := make([]ncxNavPoint, 0, len(doc.Pages))
	for i, pg := range doc.Pages {
		href := strings.TrimSpace(pg.Href)
		href = strings.TrimPrefix(cleanOPFRef("item", href), "item/")
		if href == "" {
			href = "#"
		}
		navPoints = append(navPoints, ncxNavPoint{
			ID:        fmt.Sprintf("navPoint-%d", i+1),
			PlayOrder: i + 1,
			NavLabel:  ncxNavLabel{Text: fmt.Sprintf("Page %d", i+1)},
			Content:   ncxContent{Src: href},
		})
	}
	if len(navPoints) == 0 {
		navPoints = append(navPoints, ncxNavPoint{
			ID:        "navPoint-1",
			PlayOrder: 1,
			NavLabel:  ncxNavLabel{Text: "Start"},
			Content:   ncxContent{Src: "#"},
		})
	}

	ncx := ncxDocument{
		Head: ncxHead{
			Metas: []ncxMeta{{Name: "dtb:uid", Content: identifier}},
		},
		DocTitle: ncxDocTitle{Text: metadata.Title},
		NavMap:   ncxNavMap{NavPoints: navPoints},
	}

	b, err := xml.MarshalIndent(ncx, "", "  ")
	if err != nil {
		return "", err
	}
	return xml.Header + string(b), nil
}
