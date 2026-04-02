// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestDecode_MinimalValid(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	doc, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if doc.Title != "Test Book" {
		t.Fatalf("unexpected title: %s", doc.Title)
	}
	if doc.Direction != "rtl" {
		t.Fatalf("unexpected direction: %s", doc.Direction)
	}
	if len(doc.Pages) != 1 {
		t.Fatalf("unexpected pages len: %d", len(doc.Pages))
	}
	if len(doc.Assets) != 2 {
		t.Fatalf("unexpected assets len: %d", len(doc.Assets))
	}
}

func TestDecode_PrePaginatedViewport(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true, prePaginated: true, withViewport: true})
	doc, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !doc.IsPrePaginated() {
		t.Fatal("expected pre-paginated layout")
	}
	if doc.Pages[0].Width != 1200 || doc.Pages[0].Height != 1600 {
		t.Fatalf("unexpected viewport: %dx%d", doc.Pages[0].Width, doc.Pages[0].Height)
	}
}

func TestDecode_PageHrefAndExtractReferencedImagesFromSpine(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{
		mimetypeFirst:    true,
		prePaginated:     true,
		withViewport:     true,
		embeddedImageSrc: "../image/p-001.jpg",
	})
	doc, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if doc.Pages[0].Href != "item/xhtml/p-001.xhtml" {
		t.Fatalf("unexpected page href: %s", doc.Pages[0].Href)
	}

	refs, err := doc.ExtractReferencedImagesFromSpine()
	if err != nil {
		t.Fatalf("ExtractReferencedImagesFromSpine failed: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("unexpected refs len: %d", len(refs))
	}
	if refs[0].Href != "item/image/p-001.jpg" {
		t.Fatalf("unexpected image href: %s", refs[0].Href)
	}
	if refs[0].Asset == nil || refs[0].Asset.ID != "p-001" {
		t.Fatalf("unexpected image asset: %#v", refs[0].Asset)
	}
	if refs[0].Page != doc.Pages[0] {
		t.Fatal("expected reference to point to first page")
	}
}

func TestDecode_MimetypeNotFirst(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: false})
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if !errors.Is(err, ErrMimeTypeNotFirst) {
		t.Fatalf("expected ErrMimeTypeNotFirst, got: %v", err)
	}
}

func TestDecode_MimetypeCompressed(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true, mimetypeDeflate: true})
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if !errors.Is(err, ErrInvalidMimeType) {
		t.Fatalf("expected ErrInvalidMimeType, got: %v", err)
	}
}

func TestDecode_EBPAJImageNamingViolation(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true, imageHref: "item/image/foo.jpg"})
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)), WithCompliance(LevelEBPAJ))
	if err == nil {
		t.Fatal("expected compliance error")
	}
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected DecodeError, got: %T", err)
	}
	if de.Rule != "image-naming" {
		t.Fatalf("unexpected rule: %s", de.Rule)
	}
}

func TestDecode_WarningsCollected(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{
		mimetypeFirst:       true,
		duplicateManifestID: true,
		extraArchiveFile:    "item/image/orphan.jpg",
	})
	doc, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(doc.Warnings) == 0 {
		t.Fatal("expected warnings to be collected")
	}

	if !containsWarning(doc.Warnings, "duplicate manifest id \"p-001\"") {
		t.Fatalf("expected duplicate manifest id warning, got: %#v", doc.Warnings)
	}
	if !containsWarning(doc.Warnings, "archive file \"item/image/orphan.jpg\" is not declared in manifest") {
		t.Fatalf("expected orphan file warning, got: %#v", doc.Warnings)
	}
}

type minimalEPUBConfig struct {
	mimetypeFirst       bool
	mimetypeDeflate     bool
	prePaginated        bool
	withViewport        bool
	imageHref           string
	embeddedImageSrc    string
	spineIDRef          string
	omitImageFile       bool
	duplicateManifestID bool
	extraArchiveFile    string
}

func containsWarning(warnings []string, want string) bool {
	for _, w := range warnings {
		if strings.Contains(w, want) {
			return true
		}
	}
	return false
}

func TestDecode_MissingManifestPhysicalFileStructuredError(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true, omitImageFile: true})
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if err == nil {
		t.Fatal("expected manifest-physical-existence error")
	}
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected DecodeError, got: %T", err)
	}
	if de.Rule != "manifest-physical-existence" {
		t.Fatalf("unexpected rule: %s", de.Rule)
	}
	var missing *ErrManifestPhysicalMissing
	if !errors.As(err, &missing) {
		t.Fatalf("expected ErrManifestPhysicalMissing, got: %v", err)
	}
	if missing.Href != "item/image/p-001.jpg" {
		t.Fatalf("unexpected missing href: %s", missing.Href)
	}
}

func TestDecode_UnknownSpineIDRefStructuredError(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true, spineIDRef: "missing-item"})
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if err == nil {
		t.Fatal("expected spine-idref error")
	}
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected DecodeError, got: %T", err)
	}
	if de.Rule != "spine-idref" {
		t.Fatalf("unexpected rule: %s", de.Rule)
	}
	var unknown *ErrSpineUnknownIDRef
	if !errors.As(err, &unknown) {
		t.Fatalf("expected ErrSpineUnknownIDRef, got: %v", err)
	}
	if unknown.IDRef != "missing-item" {
		t.Fatalf("unexpected missing idref: %s", unknown.IDRef)
	}
}

func TestDecode_WithMaxAssetCount_Exceeded(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)), WithMaxAssetCount(3))
	if err == nil {
		t.Fatal("expected max asset count error")
	}

	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected DecodeError, got: %T", err)
	}
	if de.Rule != "max-asset-count" {
		t.Fatalf("unexpected rule: %s", de.Rule)
	}

	var exceeded *ErrMaxAssetCountExceeded
	if !errors.As(err, &exceeded) {
		t.Fatalf("expected ErrMaxAssetCountExceeded, got: %v", err)
	}
	if exceeded.Limit != 3 {
		t.Fatalf("unexpected limit: %d", exceeded.Limit)
	}
}

func TestDecode_WithMaxTotalUncompressedSize_Exceeded(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)), WithMaxTotalUncompressedSize(200))
	if err == nil {
		t.Fatal("expected max total uncompressed size error")
	}

	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected DecodeError, got: %T", err)
	}
	if de.Rule != "max-total-uncompressed-size" {
		t.Fatalf("unexpected rule: %s", de.Rule)
	}

	var exceeded *ErrMaxTotalUncompressedSizeExceeded
	if !errors.As(err, &exceeded) {
		t.Fatalf("expected ErrMaxTotalUncompressedSizeExceeded, got: %v", err)
	}
	if exceeded.Limit != 200 {
		t.Fatalf("unexpected limit: %d", exceeded.Limit)
	}
}

func TestDecode_WithMaxIndividualAssetSize_Exceeded(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)), WithMaxIndividualAssetSize(200))
	if err == nil {
		t.Fatal("expected max individual asset size error")
	}

	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected DecodeError, got: %T", err)
	}
	if de.Rule != "max-individual-asset-size" {
		t.Fatalf("unexpected rule: %s", de.Rule)
	}

	var exceeded *ErrMaxIndividualAssetSizeExceeded
	if !errors.As(err, &exceeded) {
		t.Fatalf("expected ErrMaxIndividualAssetSizeExceeded, got: %v", err)
	}
	if exceeded.Limit != 200 {
		t.Fatalf("unexpected limit: %d", exceeded.Limit)
	}
}

func TestDecode_WithResourceLimits_AllowsWithinRange(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	_, err := Decode(
		bytes.NewReader(epubData),
		int64(len(epubData)),
		WithMaxAssetCount(10),
		WithMaxTotalUncompressedSize(10_000),
		WithMaxIndividualAssetSize(5_000),
	)
	if err != nil {
		t.Fatalf("Decode failed within configured limits: %v", err)
	}
}

func makeMinimalEPUB(t *testing.T, cfg minimalEPUBConfig) []byte {
	t.Helper()
	if cfg.imageHref == "" {
		cfg.imageHref = "item/image/p-001.jpg"
	}
	if cfg.spineIDRef == "" {
		cfg.spineIDRef = "xhtml-1"
	}
	manifestImageHref := strings.TrimPrefix(cfg.imageHref, "item/")
	if !cfg.mimetypeFirst && cfg.mimetypeDeflate {
		// allowed; this helper intentionally supports both toggles
	}

	layout := "reflowable"
	if cfg.prePaginated {
		layout = "pre-paginated"
	}

	extraManifestItem := ""
	if cfg.duplicateManifestID {
		extraManifestItem = `
		<item id="p-001" href="image/p-dup.jpg" media-type="image/jpeg"/>`
	}

	xhtml := `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
  <title>p1</title>`
	if cfg.withViewport {
		xhtml += `
  <meta name="viewport" content="width=1200,height=1600"/>`
	}
	xhtml += `
</head>
<body><p>hello</p>`
	if cfg.embeddedImageSrc != "" {
		xhtml += `
<img src="` + cfg.embeddedImageSrc + `" alt="embedded"/>`
	}
	xhtml += `</body>
</html>`

	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <meta property="rendition:layout">` + layout + `</meta>
  </metadata>
  <manifest>
		<item id="xhtml-1" href="xhtml/p-001.xhtml" media-type="application/xhtml+xml"/>
		<item id="p-001" href="` + manifestImageHref + `" media-type="image/jpeg"/>
		` + extraManifestItem + `
  </manifest>
  <spine page-progression-direction="rtl">
	    <itemref idref="` + cfg.spineIDRef + `" properties="page-spread-right"/>
  </spine>
</package>`

	container := `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="item/standard.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeMimetype := func() {
		method := zip.Store
		if cfg.mimetypeDeflate {
			method = zip.Deflate
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: uint16(method)})
		if err != nil {
			t.Fatalf("create mimetype: %v", err)
		}
		if _, err := w.Write([]byte(mimeTypeValue)); err != nil {
			t.Fatalf("write mimetype: %v", err)
		}
	}

	writeOtherFirst := func() {
		w, err := zw.Create("META-INF/container.xml")
		if err != nil {
			t.Fatalf("create container: %v", err)
		}
		if _, err := w.Write([]byte(container)); err != nil {
			t.Fatalf("write container: %v", err)
		}
	}

	if cfg.mimetypeFirst {
		writeMimetype()
	} else {
		writeOtherFirst()
		writeMimetype()
	}

	if cfg.mimetypeFirst {
		writeOtherFirst()
	}

	if w, err := zw.Create("item/standard.opf"); err != nil {
		t.Fatalf("create opf: %v", err)
	} else if _, err := w.Write([]byte(opf)); err != nil {
		t.Fatalf("write opf: %v", err)
	}
	if w, err := zw.Create("item/xhtml/p-001.xhtml"); err != nil {
		t.Fatalf("create xhtml: %v", err)
	} else if _, err := w.Write([]byte(xhtml)); err != nil {
		t.Fatalf("write xhtml: %v", err)
	}
	if !cfg.omitImageFile {
		if w, err := zw.Create(cfg.imageHref); err != nil {
			t.Fatalf("create image: %v", err)
		} else if _, err := w.Write([]byte("fake-jpeg")); err != nil {
			t.Fatalf("write image: %v", err)
		}
	}
	if cfg.duplicateManifestID {
		if w, err := zw.Create("item/image/p-dup.jpg"); err != nil {
			t.Fatalf("create duplicate image: %v", err)
		} else if _, err := w.Write([]byte("fake-jpeg-dup")); err != nil {
			t.Fatalf("write duplicate image: %v", err)
		}
	}
	if cfg.extraArchiveFile != "" {
		if w, err := zw.Create(cfg.extraArchiveFile); err != nil {
			t.Fatalf("create extra archive file: %v", err)
		} else if _, err := w.Write([]byte("orphan")); err != nil {
			t.Fatalf("write extra archive file: %v", err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
