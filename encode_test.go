// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

func TestEncode_MimetypeFirstStored(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Demo"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
		Pages: []*Page{{
			Order:   0,
			AssetID: "p-001",
			Spread:  "right",
		}},
		Assets: map[string]*Asset{
			"item/image/p-001.jpg": {
				ID:       "p-001",
				MimeType: "image/jpeg",
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader([]byte("fake-image"))), nil
				},
			},
		},
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader failed: %v", err)
	}
	if len(zr.File) == 0 {
		t.Fatal("expected zip entries")
	}
	if zr.File[0].Name != "mimetype" {
		t.Fatalf("first entry must be mimetype, got: %s", zr.File[0].Name)
	}
	if zr.File[0].Method != zip.Store {
		t.Fatalf("mimetype must be stored, got method: %d", zr.File[0].Method)
	}
}

func TestEncodeDecode_RoundTripBasic(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "RoundTrip"},
		Direction: "ltr",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	page, asset, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right")
	if err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}
	if page.AssetID == asset.ID {
		t.Fatalf("page asset should point to wrapper, got image id: %s", page.AssetID)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.Metadata.Title != "RoundTrip" {
		t.Fatalf("unexpected title: %s", decoded.Metadata.Title)
	}
	if decoded.Layout != LayoutPrePaginated {
		t.Fatalf("unexpected layout: %v", decoded.Layout)
	}
	if len(decoded.Pages) != 1 {
		t.Fatalf("unexpected pages len: %d", len(decoded.Pages))
	}
	if len(decoded.Assets) != 3 {
		t.Fatalf("unexpected assets len: %d", len(decoded.Assets))
	}
}

func TestEncode_GeneratesNavigationDocument(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Nav Test", Identifier: "E-0001"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	if _, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right"); err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	navContent := readZipEntry(t, out.Bytes(), "item/nav.xhtml")
	if !strings.Contains(navContent, `<nav epub:type="toc"`) {
		t.Fatalf("navigation toc is missing: %s", navContent)
	}
	if !strings.Contains(navContent, `<nav epub:type="landmarks"`) {
		t.Fatalf("navigation landmarks is missing: %s", navContent)
	}
	if !strings.Contains(navContent, `<h1>Navigation</h1>`) {
		t.Fatalf("navigation default title is missing: %s", navContent)
	}

	opfContent := readZipEntry(t, out.Bytes(), "item/standard.opf")
	if !strings.Contains(opfContent, `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav">`) {
		t.Fatalf("opf nav manifest item missing: %s", opfContent)
	}
}

func TestEncode_WithLegacyTOC_GeneratesNCXWithMatchingUID(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Legacy TOC", Identifier: "E-2026-0002"},
		Direction: "ltr",
		Layout:    LayoutReflowable,
	}
	png := testPNGBytes()
	if _, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "none"); err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc, WithLegacyTOC()); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	opfContent := readZipEntry(t, out.Bytes(), "item/standard.opf")
	if !strings.Contains(opfContent, `<dc:identifier id="pub-id">E-2026-0002</dc:identifier>`) {
		t.Fatalf("opf identifier missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav">`) {
		t.Fatalf("opf nav.xhtml item missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml">`) {
		t.Fatalf("opf ncx item missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<spine page-progression-direction="ltr" toc="ncx">`) {
		t.Fatalf("opf spine toc attribute missing: %s", opfContent)
	}

	ncxContent := readZipEntry(t, out.Bytes(), "item/toc.ncx")
	if !strings.Contains(ncxContent, `<meta name="dtb:uid" content="E-2026-0002"/>`) {
		t.Fatalf("ncx dtb:uid mismatch: %s", ncxContent)
	}
}

func TestEncode_Issue4AdvancedMetadata(t *testing.T) {
	doc := &Document{
		Metadata: Metadata{
			Title:             "Main Title",
			TitleFileAs:       "Main Title Yomi",
			IdentifierID:      "bw-ecode",
			Identifier:        "12345678901234567890",
			Creators:          []Creator{{Name: "Author A", FileAs: "Author A Yomi"}},
			EBPAJGuideVersion: "1.2.0",
			KADOKAWAVersion:   "2.0",
		},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	if _, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right"); err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	opfContent := readZipEntry(t, out.Bytes(), "item/standard.opf")
	if !strings.Contains(opfContent, `<package version="3.0" xmlns="http://www.idpf.org/2007/opf" unique-identifier="bw-ecode"`) {
		t.Fatalf("opf unique-identifier is missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<dc:identifier id="bw-ecode">12345678901234567890</dc:identifier>`) {
		t.Fatalf("opf bw-ecode identifier is missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<meta property="file-as" refines="#title">Main Title Yomi</meta>`) {
		t.Fatalf("title file-as is missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<dc:creator id="creator-1">Author A</dc:creator>`) {
		t.Fatalf("creator is missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<meta property="file-as" refines="#creator-1">Author A Yomi</meta>`) {
		t.Fatalf("creator file-as is missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<meta property="ebpaj:guide-version">1.2.0</meta>`) {
		t.Fatalf("ebpaj:guide-version is missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<meta property="kadokawa:version">2.0</meta>`) {
		t.Fatalf("kadokawa:version is missing: %s", opfContent)
	}
	if !regexp.MustCompile(`<meta property="dcterms:modified">[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z</meta>`).MatchString(opfContent) {
		t.Fatalf("dcterms:modified format is invalid: %s", opfContent)
	}
}

func TestEncode_RenditionSpreadAndOrientation_Defaults(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Spread Test"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	if _, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right"); err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	opfContent := readZipEntry(t, out.Bytes(), "item/standard.opf")
	if !strings.Contains(opfContent, `<meta property="rendition:spread">landscape</meta>`) {
		t.Fatalf("rendition:spread default missing:\n%s", opfContent)
	}
	if !strings.Contains(opfContent, `<meta property="rendition:orientation">auto</meta>`) {
		t.Fatalf("rendition:orientation default missing:\n%s", opfContent)
	}
}

func TestEncode_RenditionSpreadAndOrientation_Explicit(t *testing.T) {
	doc := &Document{
		Metadata: Metadata{
			Title:                "Spread Explicit",
			RenditionSpread:      "none",
			RenditionOrientation: "portrait",
		},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	if _, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right"); err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	opfContent := readZipEntry(t, out.Bytes(), "item/standard.opf")
	if !strings.Contains(opfContent, `<meta property="rendition:spread">none</meta>`) {
		t.Fatalf("rendition:spread explicit missing:\n%s", opfContent)
	}
	if !strings.Contains(opfContent, `<meta property="rendition:orientation">portrait</meta>`) {
		t.Fatalf("rendition:orientation explicit missing:\n%s", opfContent)
	}
}

func TestEncodeDecode_RenditionSpreadAndOrientation_RoundTrip(t *testing.T) {
	doc := &Document{
		Metadata: Metadata{
			Title:                "RoundTrip Spread",
			RenditionSpread:      "both",
			RenditionOrientation: "landscape",
		},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	if _, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right"); err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.Metadata.RenditionSpread != "both" {
		t.Fatalf("unexpected rendition:spread: %s", decoded.Metadata.RenditionSpread)
	}
	if decoded.Metadata.RenditionOrientation != "landscape" {
		t.Fatalf("unexpected rendition:orientation: %s", decoded.Metadata.RenditionOrientation)
	}
}

func readZipEntry(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader failed: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s failed: %v", name, err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read zip entry %s failed: %v", name, err)
		}
		return string(b)
	}
	t.Fatalf("zip entry not found: %s", name)
	return ""
}

func testPNGBytes() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func TestEncode_CoverImageAutoFallback(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Cover Auto"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	_, asset, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right")
	if err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	opfContent := readZipEntry(t, out.Bytes(), "item/standard.opf")
	expected := `<item id="` + asset.ID + `" href="image/` + asset.ID + `.png" media-type="image/png" properties="cover-image"></item>`
	if !strings.Contains(opfContent, expected) {
		t.Fatalf("cover-image property missing in manifest:\n%s", opfContent)
	}
	if !strings.Contains(opfContent, `<meta name="cover" content="`+asset.ID+`"></meta>`) {
		t.Fatalf("cover meta missing in metadata:\n%s", opfContent)
	}
}

func TestEncode_CoverImageExplicit(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Cover Explicit"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	// Add two pages; explicitly set the second as cover.
	_, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right")
	if err != nil {
		t.Fatalf("AddPageWithAsset 1 failed: %v", err)
	}
	_, asset2, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "left")
	if err != nil {
		t.Fatalf("AddPageWithAsset 2 failed: %v", err)
	}
	doc.Metadata.CoverAssetID = asset2.ID

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	opfContent := readZipEntry(t, out.Bytes(), "item/standard.opf")
	expected := `<item id="` + asset2.ID + `" href="image/` + asset2.ID + `.png" media-type="image/png" properties="cover-image"></item>`
	if !strings.Contains(opfContent, expected) {
		t.Fatalf("cover-image property missing for explicit cover asset:\n%s", opfContent)
	}
	if !strings.Contains(opfContent, `<meta name="cover" content="`+asset2.ID+`"></meta>`) {
		t.Fatalf("cover meta missing for explicit cover asset:\n%s", opfContent)
	}

	// Verify the first asset does NOT have cover-image.
	if strings.Contains(opfContent, `<item id="p-001" href="image/p-001.png" media-type="image/png" properties="cover-image"`) {
		t.Fatalf("first asset should not have cover-image property:\n%s", opfContent)
	}
}

func TestEncode_CoverImageNavLandmark(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Cover Nav"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	_, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right")
	if err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	navContent := readZipEntry(t, out.Bytes(), "item/nav.xhtml")
	// The cover landmark and bodymatter landmark should both point to the page wrapper.
	if !strings.Contains(navContent, `epub:type="cover"`) {
		t.Fatalf("cover landmark missing:\n%s", navContent)
	}
	// Cover and bodymatter point to the same single page.
	coverRe := regexp.MustCompile(`epub:type="cover" href="([^"]+)"`)
	bodyRe := regexp.MustCompile(`epub:type="bodymatter" href="([^"]+)"`)
	coverMatch := coverRe.FindStringSubmatch(navContent)
	bodyMatch := bodyRe.FindStringSubmatch(navContent)
	if len(coverMatch) < 2 || len(bodyMatch) < 2 {
		t.Fatalf("could not find cover/bodymatter hrefs:\n%s", navContent)
	}
	if coverMatch[1] != bodyMatch[1] {
		t.Fatalf("cover and bodymatter should point to same page: cover=%q body=%q", coverMatch[1], bodyMatch[1])
	}
}

func TestEncodeDecode_CoverImageRoundTrip(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Cover RoundTrip"},
		Direction: "ltr",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	_, asset, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right")
	if err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}
	doc.Metadata.CoverAssetID = asset.ID

	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.Metadata.CoverAssetID != asset.ID {
		t.Fatalf("CoverAssetID mismatch: got %q, want %q", decoded.Metadata.CoverAssetID, asset.ID)
	}
}

func TestSetCover(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "SetCover Test"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	// Add a body page first.
	_, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right")
	if err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	// Add cover via SetCover; it should be inserted at front.
	coverPage, coverAsset, err := doc.SetCover(bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("SetCover failed: %v", err)
	}
	if coverPage.Type != PageTypeCover {
		t.Fatalf("cover page Type = %q, want %q", coverPage.Type, PageTypeCover)
	}
	if doc.Metadata.CoverAssetID != coverAsset.ID {
		t.Fatalf("CoverAssetID = %q, want %q", doc.Metadata.CoverAssetID, coverAsset.ID)
	}
	if doc.Pages[0] != coverPage {
		t.Fatalf("cover page should be first in Pages")
	}
	if doc.Pages[0].Order != 0 {
		t.Fatalf("cover page Order = %d, want 0", doc.Pages[0].Order)
	}
	if doc.Pages[1].Order != 1 {
		t.Fatalf("body page Order = %d, want 1", doc.Pages[1].Order)
	}

	// Encode and decode; Page.Type must survive via cover-image detection.
	var out bytes.Buffer
	if err := Encode(&out, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := Decode(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.Metadata.CoverAssetID != coverAsset.ID {
		t.Fatalf("decoded CoverAssetID = %q, want %q", decoded.Metadata.CoverAssetID, coverAsset.ID)
	}
	foundCover := false
	for _, pg := range decoded.Pages {
		if pg.Type == PageTypeCover {
			foundCover = true
			break
		}
	}
	if !foundCover {
		t.Fatal("no page with PageTypeCover found after decode")
	}
}

func TestEncode_WithPreflightCompliance_NoWarningsForValidDoc(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Valid"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	png := testPNGBytes()
	if _, _, err := doc.AddPageWithAsset(bytes.NewReader(png), int64(len(png)), "right"); err != nil {
		t.Fatalf("AddPageWithAsset failed: %v", err)
	}

	var warnings []string
	var out bytes.Buffer
	err := Encode(&out, doc,
		WithEncodePreflightCompliance(LevelKADOKAWA),
		WithEncodeWarningCollector(func(w string) { warnings = append(warnings, w) }),
	)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", warnings)
	}
}

func TestEncode_WithPreflightCompliance_ImageSizeWarning(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Large Image"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
		Assets: map[string]*Asset{
			"item/image/p-001.jpg": {
				ID:       "p-001",
				MimeType: "image/jpeg",
				Size:     5 * 1024 * 1024, // 5MB, exceeds 4MB limit
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
				},
			},
		},
		Pages: []*Page{{Order: 0, AssetID: "p-001", Spread: "right"}},
	}

	var warnings []string
	var out bytes.Buffer
	err := Encode(&out, doc,
		WithEncodePreflightCompliance(LevelEBPAJ),
		WithEncodeWarningCollector(func(w string) { warnings = append(warnings, w) }),
	)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "file size") && strings.Contains(w, "exceeds") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected file-size warning, got: %v", warnings)
	}
}

func TestEncode_WithPreflightCompliance_DirectoryLayoutWarning(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Bad Layout"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
		Assets: map[string]*Asset{
			"baddir/p-001.jpg": {
				ID:       "p-001",
				MimeType: "image/jpeg",
				Size:     100,
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
				},
			},
		},
		Pages: []*Page{{Order: 0, AssetID: "p-001", Spread: "right"}},
	}

	var warnings []string
	var out bytes.Buffer
	err := Encode(&out, doc,
		WithEncodePreflightCompliance(LevelKADOKAWA),
		WithEncodeWarningCollector(func(w string) { warnings = append(warnings, w) }),
	)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "directory layout") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected directory-layout warning, got: %v", warnings)
	}
}

func TestEncode_WithPreflightCompliance_NamingWarning(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Bad Naming"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
		Assets: map[string]*Asset{
			"item/image/BADNAME.jpg": {
				ID:       "p-001",
				MimeType: "image/jpeg",
				Size:     100,
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
				},
			},
		},
		Pages: []*Page{{Order: 0, AssetID: "p-001", Spread: "right"}},
	}

	var warnings []string
	var out bytes.Buffer
	err := Encode(&out, doc,
		WithEncodePreflightCompliance(LevelEBPAJ),
		WithEncodeWarningCollector(func(w string) { warnings = append(warnings, w) }),
	)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "naming pattern") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected naming-pattern warning, got: %v", warnings)
	}
}

func TestEncode_WithPreflightCompliance_FlexibleNoWarnings(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "Flexible"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
		Assets: map[string]*Asset{
			"baddir/BADNAME.jpg": {
				ID:       "p-001",
				MimeType: "image/jpeg",
				Size:     100,
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
				},
			},
		},
		Pages: []*Page{{Order: 0, AssetID: "p-001", Spread: "right"}},
	}

	var warnings []string
	var out bytes.Buffer
	err := Encode(&out, doc,
		WithEncodePreflightCompliance(LevelFlexible),
		WithEncodeWarningCollector(func(w string) { warnings = append(warnings, w) }),
	)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("flexible mode should produce no warnings, got: %v", warnings)
	}
}

func TestEncode_WithPreflightCompliance_NoCollectorDiscards(t *testing.T) {
	doc := &Document{
		Metadata:  Metadata{Title: "No Collector"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
		Assets: map[string]*Asset{
			"baddir/BADNAME.jpg": {
				ID:       "p-001",
				MimeType: "image/jpeg",
				Size:     100,
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
				},
			},
		},
		Pages: []*Page{{Order: 0, AssetID: "p-001", Spread: "right"}},
	}

	var out bytes.Buffer
	// Preflight enabled but no collector — should not panic.
	err := Encode(&out, doc, WithEncodePreflightCompliance(LevelKADOKAWA))
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
}
