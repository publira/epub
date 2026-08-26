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
	if doc.Metadata.Title != "Test Book" {
		t.Fatalf("unexpected title: %s", doc.Metadata.Title)
	}
	if doc.Title != "Test Book" {
		t.Fatalf("unexpected legacy title: %s", doc.Title)
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

func TestDecode_Issue4AdvancedMetadata(t *testing.T) {
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf" unique-identifier="bw-ecode" prefix="rendition: http://www.idpf.org/vocab/rendition/# dcterms: http://purl.org/dc/terms/ ebpaj: https://www.ebpaj.jp/ kadokawa: https://www.kadokawa.co.jp/">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title id="title">Main Title</dc:title>
    <dc:identifier id="bw-ecode">12345678901234567890</dc:identifier>
    <dc:creator id="creator-1">Author A</dc:creator>
    <meta property="file-as" refines="#title">Main Title Yomi</meta>
    <meta property="file-as" refines="#creator-1">Author A Yomi</meta>
    <meta property="rendition:layout">pre-paginated</meta>
    <meta property="dcterms:modified">2026-04-02T12:34:56Z</meta>
    <meta property="ebpaj:guide-version">1.2.0</meta>
    <meta property="kadokawa:version">2.0</meta>
  </metadata>
  <manifest>
    <item id="xhtml-1" href="xhtml/p-001.xhtml" media-type="application/xhtml+xml"/>
    <item id="p-001" href="image/p-001.jpg" media-type="image/jpeg"/>
  </manifest>
  <spine page-progression-direction="rtl">
    <itemref idref="xhtml-1" properties="page-spread-right"/>
  </spine>
</package>`

	container := `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="item/standard.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
	xhtml := `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>p1</title><meta name="viewport" content="width=1200,height=1600"/></head><body><p>hello</p></body></html>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	write := func(name string, method uint16, body string) {
		t.Helper()
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("mimetype", zip.Store, "application/epub+zip")
	write("META-INF/container.xml", zip.Deflate, container)
	write("item/standard.opf", zip.Deflate, opf)
	write("item/xhtml/p-001.xhtml", zip.Deflate, xhtml)
	write("item/image/p-001.jpg", zip.Deflate, "fake-jpeg")

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	doc, err := Decode(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if doc.Metadata.IdentifierID != "bw-ecode" {
		t.Fatalf("unexpected metadata identifier id: %s", doc.Metadata.IdentifierID)
	}
	if doc.Metadata.Identifier != "12345678901234567890" {
		t.Fatalf("unexpected metadata identifier: %s", doc.Metadata.Identifier)
	}
	if doc.Identifier != "12345678901234567890" {
		t.Fatalf("unexpected legacy identifier: %s", doc.Identifier)
	}
	if doc.Metadata.Title != "Main Title" {
		t.Fatalf("unexpected metadata title: %s", doc.Metadata.Title)
	}
	if doc.Metadata.TitleFileAs != "Main Title Yomi" {
		t.Fatalf("unexpected metadata title file-as: %s", doc.Metadata.TitleFileAs)
	}
	if len(doc.Metadata.Creators) != 1 {
		t.Fatalf("unexpected metadata creators len: %d", len(doc.Metadata.Creators))
	}
	if doc.Metadata.Creators[0].Name != "Author A" || doc.Metadata.Creators[0].FileAs != "Author A Yomi" {
		t.Fatalf("unexpected metadata creator: %#v", doc.Metadata.Creators[0])
	}
	if doc.Metadata.EBPAJGuideVersion != "1.2.0" {
		t.Fatalf("unexpected metadata ebpaj guide version: %s", doc.Metadata.EBPAJGuideVersion)
	}
	if doc.Metadata.KADOKAWAVersion != "2.0" {
		t.Fatalf("unexpected metadata kadokawa version: %s", doc.Metadata.KADOKAWAVersion)
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
	var missing *ManifestPhysicalMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("expected ManifestPhysicalMissingError, got: %v", err)
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
	var unknown *SpineUnknownIDRefError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected SpineUnknownIDRefError, got: %v", err)
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

	var exceeded *MaxAssetCountExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("expected MaxAssetCountExceededError, got: %v", err)
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

	var exceeded *MaxTotalUncompressedSizeExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("expected MaxTotalUncompressedSizeExceededError, got: %v", err)
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

	var exceeded *MaxIndividualAssetSizeExceededError
	if !errors.As(err, &exceeded) {
		t.Fatalf("expected MaxIndividualAssetSizeExceededError, got: %v", err)
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
	// Both !mimetypeFirst && mimetypeDeflate is allowed;
	// this helper intentionally supports both toggles.

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
		w, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: method})
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

// --- WithValidator tests ---

// stubValidator is a test Validator that returns errors for specific hrefs.
type stubValidator struct {
	assetErrors map[string][]error
	docErrors   []error
}

func (v *stubValidator) ValidateDocument(_ *Document) []error { return v.docErrors }

func (v *stubValidator) ValidateAsset(href string, _ *Asset) []error {
	return v.assetErrors[href]
}

func TestDecode_WithValidator_FatalError(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	v := &stubValidator{
		assetErrors: map[string][]error{
			"item/image/p-001.jpg": {errors.New("image is bad")},
		},
	}
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)), WithValidator(v))
	if err == nil {
		t.Fatal("expected error from validator")
	}
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected DecodeError, got: %T", err)
	}
	if de.Rule != "validator" {
		t.Fatalf("unexpected rule: %s", de.Rule)
	}
}

func TestDecode_WithValidator_Warning(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	v := &stubValidator{
		assetErrors: map[string][]error{
			"item/image/p-001.jpg": {&ValidationWarningError{Message: "soft issue"}},
		},
	}
	doc, err := Decode(bytes.NewReader(epubData), int64(len(epubData)), WithValidator(v))
	if err != nil {
		t.Fatalf("expected no fatal error, got: %v", err)
	}
	found := false
	for _, w := range doc.Warnings {
		if strings.Contains(w, "soft issue") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected warning 'soft issue', got: %v", doc.Warnings)
	}
}

func TestDecode_WithValidator_DocumentLevelError(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	v := &stubValidator{
		docErrors: []error{errors.New("doc-level issue")},
	}
	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)), WithValidator(v))
	if err == nil {
		t.Fatal("expected error from document-level validator")
	}
}

func TestDecode_WithValidator_MultipleValidators(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	v1 := &stubValidator{
		assetErrors: map[string][]error{
			"item/image/p-001.jpg": {&ValidationWarningError{Message: "warning-from-v1"}},
		},
	}
	v2 := &stubValidator{
		assetErrors: map[string][]error{
			"item/image/p-001.jpg": {&ValidationWarningError{Message: "warning-from-v2"}},
		},
	}
	doc, err := Decode(bytes.NewReader(epubData), int64(len(epubData)), WithValidator(v1, v2))
	if err != nil {
		t.Fatalf("expected no fatal error, got: %v", err)
	}
	foundV1, foundV2 := false, false
	for _, w := range doc.Warnings {
		if strings.Contains(w, "warning-from-v1") {
			foundV1 = true
		}
		if strings.Contains(w, "warning-from-v2") {
			foundV2 = true
		}
	}
	if !foundV1 || !foundV2 {
		t.Fatalf("expected warnings from both validators, got: %v", doc.Warnings)
	}
}

func TestDecode_WithValidator_NoValidators(t *testing.T) {
	epubData := makeMinimalEPUB(t, minimalEPUBConfig{mimetypeFirst: true})
	doc, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if doc.Metadata.Title != "Test Book" {
		t.Fatalf("unexpected title: %s", doc.Metadata.Title)
	}
}
