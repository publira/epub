// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func TestEncode_MimetypeFirstStored(t *testing.T) {
	doc := &Document{
		Title:     "Demo",
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
		Title:     "RoundTrip",
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
	if decoded.Title != "RoundTrip" {
		t.Fatalf("unexpected title: %s", decoded.Title)
	}
	if decoded.Layout != LayoutPrePaginated {
		t.Fatalf("unexpected layout: %v", decoded.Layout)
	}
	if len(decoded.Pages) != 1 {
		t.Fatalf("unexpected pages len: %d", len(decoded.Pages))
	}
	if len(decoded.Assets) != 1 {
		t.Fatalf("unexpected assets len: %d", len(decoded.Assets))
	}
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
