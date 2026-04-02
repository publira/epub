// SPDX-License-Identifier: Apache-2.0

package epub

import (
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
	Title     string
	Identifier string
	Direction string // "rtl" (right-to-left) or "ltr".
	Layout    LayoutType
	Pages     []*Page
	Assets    map[string]*Asset
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
// If page creation fails after asset registration, the asset is rolled back.
func (d *Document) AddPageWithAsset(r io.ReaderAt, size int64, spread string) (*Page, *Asset, error) {
	mimeType, width, height, err := detectAssetMeta(r, size)
	if err != nil {
		return nil, nil, err
	}

	assetHref, asset, err := d.AddAsset(mimeType, r, size)
	if err != nil {
		return nil, nil, err
	}

	page, err := d.AddPage(width, height, spread)
	if err != nil {
		delete(d.Assets, assetHref)
		return nil, nil, err
	}
	page.AssetID = asset.ID
	page.Href = assetHref

	return page, asset, nil
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

func readerAtSize(r io.ReaderAt) (int64, error) {
	if s, ok := r.(interface{ Size() int64 }); ok {
		return s.Size(), nil
	}
	if l, ok := r.(interface{ Len() int }); ok {
		return int64(l.Len()), nil
	}

	one := make([]byte, 1)
	var hi int64 = 1
	for {
		_, err := r.ReadAt(one, hi-1)
		if err == nil {
			if hi > (1 << 62) {
				return 0, ErrCannotInferAssetSize
			}
			hi *= 2
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return 0, err
	}

	lo := hi / 2
	for lo+1 < hi {
		mid := lo + (hi-lo)/2
		_, err := r.ReadAt(one, mid-1)
		if err == nil {
			lo = mid
			continue
		}
		if errors.Is(err, io.EOF) {
			hi = mid
			continue
		}
		return 0, err
	}
	return lo, nil
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

// Page is a single reading-order entry in spine.
type Page struct {
	Order   int
	AssetID string
	Href    string
	Width   int
	Height  int
	Spread  string // "left", "right", "center", "none".
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
	defer rc.Close()

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
	defer rc.Close()

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
