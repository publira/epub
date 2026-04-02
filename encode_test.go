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
	if page.AssetID != asset.ID {
		t.Fatalf("page asset mismatch: %s vs %s", page.AssetID, asset.ID)
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
	if len(decoded.Assets) != 2 {
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
	if !strings.Contains(opfContent, `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`) {
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
	if !strings.Contains(opfContent, `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`) {
		t.Fatalf("opf nav.xhtml item missing: %s", opfContent)
	}
	if !strings.Contains(opfContent, `<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`) {
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
