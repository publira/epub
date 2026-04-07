// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"path"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// Document is the normalized public model of an EPUB package.
//
// Assets uses the manifest href as the map key.
type Document struct {
	Metadata Metadata
	// Title is kept for backward compatibility. Prefer Metadata.Title for new code.
	Title string
	// Identifier is kept for backward compatibility. Prefer Metadata.Identifier for new code.
	Identifier string
	Direction  string // "rtl" (right-to-left) or "ltr".
	Layout     LayoutType
	Pages      []*Page
	Assets     map[string]*Asset
	Warnings   []string
}

// Metadata represents OPF metadata values used by this package.
type Metadata struct {
	Title                string
	TitleFileAs          string
	Identifier           string
	IdentifierID         string
	Language             string
	Creators             []Creator
	CoverAssetID         string
	RenditionSpread      string
	RenditionOrientation string
	EBPAJGuideVersion    string
	KADOKAWAVersion      string
}

// Creator represents a dc:creator entry with optional phonetic sort key.
type Creator struct {
	Name   string
	FileAs string
}

func (d *Document) effectiveMetadata() Metadata {
	if d == nil {
		return Metadata{}
	}
	m := d.Metadata
	if strings.TrimSpace(m.Title) == "" {
		m.Title = d.Title
	}
	if strings.TrimSpace(m.Identifier) == "" {
		m.Identifier = d.Identifier
	}
	if strings.TrimSpace(m.Language) == "" {
		m.Language = "en"
	}
	return m
}

// LayoutType represents rendition:layout in OPF metadata.
type LayoutType int

const (
	// LayoutUnknown means rendition:layout is missing or unsupported.
	LayoutUnknown LayoutType = iota
	// LayoutReflowable corresponds to rendition:layout="reflowable".
	LayoutReflowable
	// LayoutPrePaginated corresponds to rendition:layout="pre-paginated".
	LayoutPrePaginated
)

// String returns a human-readable layout value.
func (l LayoutType) String() string {
	switch l {
	case LayoutReflowable:
		return "reflowable"
	case LayoutPrePaginated:
		return "pre-paginated"
	default:
		return "unknown"
	}
}

// IsPrePaginated reports whether the document uses fixed-layout style pages.
func (d *Document) IsPrePaginated() bool {
	return d != nil && d.Layout == LayoutPrePaginated
}

// GetAssetByPage resolves the corresponding asset from the given page reference.
func (d *Document) GetAssetByPage(p *Page) (*Asset, error) {
	if d == nil {
		return nil, ErrNilDocument
	}
	if p == nil {
		return nil, ErrNilPage
	}
	if href := strings.TrimSpace(p.Href); href != "" {
		href = cleanOPFRef("", href)
		if a, ok := d.Assets[href]; ok && a != nil {
			if strings.TrimSpace(p.AssetID) == "" || a.ID == p.AssetID {
				return a, nil
			}
		}
		if strings.TrimSpace(p.AssetID) == "" {
			return nil, ErrAssetNotFound
		}
	}
	if strings.TrimSpace(p.AssetID) == "" {
		return nil, ErrEmptyAssetID
	}

	for _, a := range d.Assets {
		if a != nil && a.ID == p.AssetID {
			return a, nil
		}
	}
	return nil, ErrAssetNotFound
}

// ResolveSpineAssets verifies that all pages in spine can resolve to existing assets.
func (d *Document) ResolveSpineAssets() error {
	if d == nil {
		return ErrNilDocument
	}
	for i, p := range d.Pages {
		if _, err := d.GetAssetByPage(p); err != nil {
			return fmt.Errorf("page[%d]: %w", i, err)
		}
	}
	return nil
}

// AddPage appends a new page and automatically assigns reading order.
//
// For reflowable content, width/height can be 0 and spread can be empty.
// spread accepts: left, right, center, none.
func (d *Document) AddPage(width, height int, spread string) (*Page, error) {
	if d == nil {
		return nil, ErrNilDocument
	}
	assetID := d.nextAssetID()

	if spread == "" {
		spread = "none"
	}
	spread = strings.ToLower(strings.TrimSpace(spread))
	if spread != "left" && spread != "right" && spread != "center" && spread != "none" {
		return nil, ErrInvalidSpread
	}

	page := &Page{
		Order:   len(d.Pages),
		AssetID: assetID,
		Width:   width,
		Height:  height,
		Spread:  spread,
	}
	d.Pages = append(d.Pages, page)
	return page, nil
}

// AddAsset registers an asset for encoding and returns the stored pointer.
//
// The content source is io.ReaderAt + size so the stream can be reopened on demand.
// id and href are always auto-generated in specification-friendly form.
func (d *Document) AddAsset(mimeType string, r io.ReaderAt, size int64) (string, *Asset, error) {
	if d == nil {
		return "", nil, ErrNilDocument
	}
	mimeType = strings.TrimSpace(mimeType)

	if mimeType == "" {
		return "", nil, ErrEmptyMimeType
	}
	id := d.nextAssetID()
	ext, ok := fileExtFromMimeType(mimeType)
	if !ok {
		return "", nil, ErrCannotInferAssetPath
	}
	href := path.Join("item/image", fmt.Sprintf("%s.%s", id, ext))
	if r == nil {
		return "", nil, ErrNilReaderAt
	}
	if size < 0 {
		return "", nil, ErrInvalidAssetSize
	}

	if d.Assets == nil {
		d.Assets = make(map[string]*Asset)
	}
	if _, exists := d.Assets[href]; exists {
		return "", nil, ErrDuplicateAssetPath
	}
	for _, a := range d.Assets {
		if a != nil && a.ID == id {
			return "", nil, ErrDuplicateAssetID
		}
	}

	asset := &Asset{
		ID:       id,
		MimeType: mimeType,
		Size:     uint64(size),
		Open: func() (io.ReadCloser, error) {
			sr := io.NewSectionReader(r, 0, size)
			return io.NopCloser(sr), nil
		},
	}
	if err := asset.CalculateHash(); err != nil {
		return "", nil, err
	}
	d.Assets[href] = asset
	return href, asset, nil
}

// AddPageWithAsset registers an asset and appends its page in one call.
//
// mime type and viewport width/height are derived from r.
// The spine is linked to a generated XHTML wrapper to keep fixed-layout rendering
// behavior consistent across readers (including Apple Books).
// If page creation fails after asset registration, created assets are rolled back.
func (d *Document) AddPageWithAsset(r io.ReaderAt, size int64, spread string) (*Page, *Asset, error) {
	mimeType, width, height, err := detectAssetMeta(r, size)
	if err != nil {
		return nil, nil, err
	}

	assetHref, asset, err := d.AddAsset(mimeType, r, size)
	if err != nil {
		return nil, nil, err
	}

	xhtmlHref, xhtmlAsset, err := d.addXHTMLPageAsset(assetHref, asset.ID, width, height)
	if err != nil {
		delete(d.Assets, assetHref)
		return nil, nil, err
	}

	page, err := d.AddPage(width, height, spread)
	if err != nil {
		delete(d.Assets, xhtmlHref)
		delete(d.Assets, assetHref)
		return nil, nil, err
	}
	page.AssetID = xhtmlAsset.ID
	page.Href = xhtmlHref

	return page, asset, nil
}

// SetCover adds a cover image asset and its wrapper page, marking it as the cover
// in both the metadata (properties="cover-image") and the spine (PageTypeCover).
// The cover page is inserted at the beginning of Pages (Order 0).
func (d *Document) SetCover(r io.ReaderAt, size int64) (*Page, *Asset, error) {
	page, asset, err := d.AddPageWithAsset(r, size, "center")
	if err != nil {
		return nil, nil, err
	}

	d.Metadata.CoverAssetID = asset.ID
	page.Type = PageTypeCover

	// Move cover page to front of Pages.
	if len(d.Pages) > 1 {
		last := len(d.Pages) - 1
		cover := d.Pages[last]
		copy(d.Pages[1:], d.Pages[:last])
		d.Pages[0] = cover
		for i, pg := range d.Pages {
			pg.Order = i
		}
	}

	return page, asset, nil
}

func (d *Document) addXHTMLPageAsset(imageHref, imageID string, width, height int) (string, *Asset, error) {
	if d == nil {
		return "", nil, ErrNilDocument
	}
	xhtmlID := "xhtml-" + imageID
	xhtmlHref := path.Join("item/xhtml", imageID+".xhtml")
	if d.Assets == nil {
		d.Assets = make(map[string]*Asset)
	}
	if _, exists := d.Assets[xhtmlHref]; exists {
		return "", nil, ErrDuplicateAssetPath
	}
	for _, a := range d.Assets {
		if a != nil && a.ID == xhtmlID {
			return "", nil, ErrDuplicateAssetID
		}
	}

	imgSrc := path.Clean(path.Join("..", path.Base(path.Dir(imageHref)), path.Base(imageHref)))
	body, err := buildXHTMLPageWrapper(width, height, imgSrc)
	if err != nil {
		return "", nil, err
	}
	asset := &Asset{
		ID:       xhtmlID,
		MimeType: "application/xhtml+xml",
		Size:     uint64(len(body)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		},
	}
	if err := asset.CalculateHash(); err != nil {
		return "", nil, err
	}

	d.Assets[xhtmlHref] = asset
	return xhtmlHref, asset, nil
}

func buildXHTMLPageWrapper(width, height int, imgSrc string) ([]byte, error) {
	doc := xhtmlDocument{
		XMLNS:     "http://www.w3.org/1999/xhtml",
		XMLNSEpub: "http://www.idpf.org/2007/ops",
		Head: xhtmlHead{
			Title: "Page",
			Meta: xhtmlMeta{
				Name:    "viewport",
				Content: fmt.Sprintf("width=%d, height=%d", width, height),
			},
			Style: "html, body { margin: 0; padding: 0; width: 100%; height: 100%; overflow: hidden; }\n" +
				"img { display: block; width: 100%; height: 100%; object-fit: contain; }",
		},
		Body: xhtmlBody{
			Img: xhtmlImg{Src: imgSrc, Alt: ""},
		},
	}

	b, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), b...), nil
}

func detectAssetMeta(r io.ReaderAt, size int64) (mimeType string, width int, height int, err error) {
	if r == nil {
		return "", 0, 0, ErrNilReaderAt
	}
	if size <= 0 {
		return "", 0, 0, ErrInvalidAssetSize
	}

	headLen := int64(512)
	if size < headLen {
		headLen = size
	}
	head := make([]byte, headLen)
	n, err := r.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", 0, 0, err
	}
	mimeType = http.DetectContentType(head[:n])
	if !strings.HasPrefix(mimeType, "image/") {
		return "", 0, 0, ErrCannotInferAssetPath
	}

	cfg, _, err := image.DecodeConfig(io.NewSectionReader(r, 0, size))
	if err != nil {
		return "", 0, 0, err
	}
	return mimeType, cfg.Width, cfg.Height, nil
}

func (d *Document) nextAssetID() string {
	pad := 3
	if len(d.Assets) >= 999 || len(d.Pages) >= 999 {
		pad = 4
	}
	for i := 1; i < 100000; i++ {
		candidate := fmt.Sprintf("p-%0*d", pad, i)
		if !d.hasAssetID(candidate) {
			return candidate
		}
	}
	return "p-000"
}

func (d *Document) hasAssetID(id string) bool {
	for _, a := range d.Assets {
		if a != nil && a.ID == id {
			return true
		}
	}
	for _, p := range d.Pages {
		if p != nil && p.AssetID == id {
			return true
		}
	}
	return false
}

func fileExtFromMimeType(mimeType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return "jpg", true
	case "image/png":
		return "png", true
	case "image/gif":
		return "gif", true
	case "image/webp":
		return "webp", true
	default:
		return "", false
	}
}

// PageType represents semantic role of a page in the reading order.
type PageType string

const (
	// PageTypeStandard is a regular content page (default).
	PageTypeStandard PageType = ""
	// PageTypeCover marks the page as the cover page.
	PageTypeCover PageType = "cover"
	// PageTypeTOC marks the page as a table of contents page.
	PageTypeTOC PageType = "toc"
)

// Page is a single reading-order entry in spine.
type Page struct {
	Order   int
	AssetID string
	Href    string
	Width   int
	Height  int
	Spread  string   // "left", "right", "center", "none".
	Type    PageType // Semantic role of the page.
}

// Asset points to a binary object referenced from EPUB manifest.
type Asset struct {
	ID       string
	MimeType string
	Size     uint64
	Checksum string
	Open     func() (io.ReadCloser, error)
}

// SpineImageReference is an image asset referenced by a page in spine order.
type SpineImageReference struct {
	Page  *Page
	Href  string
	Asset *Asset
}

// ExtractReferencedImagesFromSpine returns image assets referenced by spine pages.
//
// Direct image pages are returned as-is. XHTML pages are scanned for img/image
// elements and their manifest-referenced image assets are resolved in reading order.
func (d *Document) ExtractReferencedImagesFromSpine() ([]SpineImageReference, error) {
	if d == nil {
		return nil, ErrNilDocument
	}

	refs := make([]SpineImageReference, 0, len(d.Pages))
	for i, page := range d.Pages {
		if page == nil {
			return nil, fmt.Errorf("page[%d]: %w", i, ErrNilPage)
		}

		asset, err := d.GetAssetByPage(page)
		if err != nil {
			return nil, fmt.Errorf("page[%d]: %w", i, err)
		}

		pageHref := strings.TrimSpace(page.Href)
		if pageHref == "" {
			pageHref = d.findAssetHref(asset)
		}

		mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
		if strings.HasPrefix(mimeType, "image/") {
			refs = append(refs, SpineImageReference{Page: page, Href: pageHref, Asset: asset})
			continue
		}
		if !strings.Contains(mimeType, "xhtml") {
			continue
		}
		if pageHref == "" {
			return nil, fmt.Errorf("page[%d]: %w", i, ErrEmptyAssetPath)
		}

		rawRefs, err := extractImageRefsFromAsset(asset)
		if err != nil {
			return nil, fmt.Errorf("page[%d]: %w", i, err)
		}
		for _, rawRef := range rawRefs {
			if !isLocalAssetRef(rawRef) {
				continue
			}
			resolved := cleanOPFRef(path.Dir(pageHref), rawRef)
			imageAsset, ok := d.Assets[resolved]
			if !ok || imageAsset == nil {
				return nil, fmt.Errorf("page[%d]: referenced image %q: %w", i, resolved, ErrAssetNotFound)
			}
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(imageAsset.MimeType)), "image/") {
				continue
			}
			refs = append(refs, SpineImageReference{Page: page, Href: resolved, Asset: imageAsset})
		}
	}

	return refs, nil
}

// CalculateHash computes SHA-256 by streaming from Open without buffering full contents.
func (a *Asset) CalculateHash() error {
	if a == nil {
		return ErrNilAsset
	}
	if a.Open == nil {
		return ErrNilAssetOpen
	}
	rc, err := a.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, rc); err != nil {
		return err
	}
	a.Checksum = hex.EncodeToString(h.Sum(nil))
	return nil
}

func (d *Document) findAssetHref(target *Asset) string {
	if d == nil || target == nil {
		return ""
	}
	for href, asset := range d.Assets {
		if asset == nil {
			continue
		}
		if asset == target || asset.ID == target.ID {
			return href
		}
	}
	return ""
}

func extractImageRefsFromAsset(asset *Asset) ([]string, error) {
	if asset == nil {
		return nil, ErrNilAsset
	}
	if asset.Open == nil {
		return nil, ErrNilAssetOpen
	}

	rc, err := asset.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	buf, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return extractImageRefsFromXHTML(string(buf))
}

func extractImageRefsFromXHTML(xhtml string) ([]string, error) {
	decoder := xml.NewDecoder(strings.NewReader(xhtml))
	refs := make([]string, 0, 1)

	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return refs, nil
			}
			return nil, err
		}

		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		switch strings.ToLower(se.Name.Local) {
		case "img":
			if ref := xmlAttrValue(se.Attr, "src"); ref != "" {
				refs = append(refs, ref)
			}
		case "image":
			if ref := xmlAttrValue(se.Attr, "href"); ref != "" {
				refs = append(refs, ref)
			}
		}
	}
}

func xmlAttrValue(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name.Local, local) {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func isLocalAssetRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") {
		return false
	}
	lower := strings.ToLower(ref)
	return !strings.HasPrefix(lower, "http://") &&
		!strings.HasPrefix(lower, "https://") &&
		!strings.HasPrefix(lower, "data:")
}
