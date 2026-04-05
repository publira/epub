// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestAddAssetAutoDerivation(t *testing.T) {
	doc := &Document{}
	r := strings.NewReader("abc")

	href, asset, err := doc.AddAsset("image/jpeg", r, int64(r.Len()))
	if err != nil {
		t.Fatalf("AddAsset returned error: %v", err)
	}
	if href != "item/image/p-001.jpg" {
		t.Fatalf("unexpected href: %s", href)
	}
	if asset.ID != "p-001" {
		t.Fatalf("unexpected asset ID: %s", asset.ID)
	}
	if asset.Checksum == "" {
		t.Fatal("checksum should be populated")
	}
	if got := len(doc.Assets); got != 1 {
		t.Fatalf("unexpected asset count: %d", got)
	}
}

func TestAddPageAutoDerivation(t *testing.T) {
	doc := &Document{}
	page, err := doc.AddPage(1600, 2560, "right")
	if err != nil {
		t.Fatalf("AddPage returned error: %v", err)
	}
	if page.Order != 0 {
		t.Fatalf("unexpected order: %d", page.Order)
	}
	if page.AssetID != "p-001" {
		t.Fatalf("unexpected page asset ID: %s", page.AssetID)
	}
}

func TestAddPageWithAssetGeneratesXHTMLWrapper(t *testing.T) {
	doc := &Document{}
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7Zk7kAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png fixture: %v", err)
	}
	r := bytes.NewReader(pngBytes)

	page, asset, err := doc.AddPageWithAsset(r, int64(r.Len()), "left")
	if err != nil {
		t.Fatalf("AddPageWithAsset returned error: %v", err)
	}
	if page.AssetID == asset.ID {
		t.Fatalf("page asset id (%s) must point to wrapper, not image (%s)", page.AssetID, asset.ID)
	}
	if page.Href != "item/xhtml/p-001.xhtml" {
		t.Fatalf("unexpected page href: %s", page.Href)
	}
	if page.Spread != "left" {
		t.Fatalf("unexpected spread: %s", page.Spread)
	}
	if page.Width != 1 || page.Height != 1 {
		t.Fatalf("unexpected viewport: %dx%d", page.Width, page.Height)
	}
	if len(doc.Assets) != 2 {
		t.Fatalf("unexpected assets len: %d", len(doc.Assets))
	}

	wrapper := doc.Assets[page.Href]
	if wrapper == nil {
		t.Fatalf("wrapper asset not found: %s", page.Href)
	}
	if wrapper.MimeType != "application/xhtml+xml" {
		t.Fatalf("unexpected wrapper mime type: %s", wrapper.MimeType)
	}
	rc, err := wrapper.Open()
	if err != nil {
		t.Fatalf("open wrapper failed: %v", err)
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read wrapper failed: %v", err)
	}
	xhtml := string(body)
	if !strings.Contains(xhtml, `name="viewport" content="width=1, height=1"`) {
		t.Fatalf("viewport meta is missing: %s", xhtml)
	}
	if !strings.Contains(xhtml, `img src="../image/p-001.png"`) {
		t.Fatalf("image ref is missing: %s", xhtml)
	}
}

func TestDocumentExtractReferencedImagesFromSpineDirectImage(t *testing.T) {
	doc := &Document{}
	pngBytes := testPNGBytes()

	page, asset, err := doc.AddPageWithAsset(bytes.NewReader(pngBytes), int64(len(pngBytes)), "right")
	if err != nil {
		t.Fatalf("AddPageWithAsset returned error: %v", err)
	}

	refs, err := doc.ExtractReferencedImagesFromSpine()
	if err != nil {
		t.Fatalf("ExtractReferencedImagesFromSpine returned error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("unexpected refs len: %d", len(refs))
	}
	if refs[0].Page != page {
		t.Fatal("expected page pointer to be preserved")
	}
	if refs[0].Asset != asset {
		t.Fatal("expected asset pointer to be preserved")
	}
	if refs[0].Href != "item/image/p-001.png" {
		t.Fatalf("unexpected href: %s", refs[0].Href)
	}
}

func TestAddAssetUnsupportedMime(t *testing.T) {
	doc := &Document{}
	r := strings.NewReader("abc")

	_, _, err := doc.AddAsset("application/octet-stream", r, int64(r.Len()))
	if !errors.Is(err, ErrCannotInferAssetPath) {
		t.Fatalf("expected ErrCannotInferAssetPath, got: %v", err)
	}
}

func TestDocumentGetAssetByPage(t *testing.T) {
	doc := &Document{}
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7Zk7kAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png fixture: %v", err)
	}

	page, asset, err := doc.AddPageWithAsset(bytes.NewReader(pngBytes), int64(len(pngBytes)), "right")
	if err != nil {
		t.Fatalf("AddPageWithAsset returned error: %v", err)
	}

	resolved, err := doc.GetAssetByPage(page)
	if err != nil {
		t.Fatalf("GetAssetByPage returned error: %v", err)
	}
	if resolved == asset {
		t.Fatal("resolved page asset should be wrapper, not returned image asset")
	}
	if resolved == nil || resolved.MimeType != "application/xhtml+xml" {
		t.Fatalf("unexpected resolved page asset: %#v", resolved)
	}
}

func TestDocumentGetAssetByPageErrors(t *testing.T) {
	tests := []struct {
		name string
		doc  *Document
		page *Page
		want error
	}{
		{
			name: "nil document",
			doc:  nil,
			page: &Page{AssetID: "p-001"},
			want: ErrNilDocument,
		},
		{
			name: "nil page",
			doc:  &Document{},
			page: nil,
			want: ErrNilPage,
		},
		{
			name: "empty asset id",
			doc:  &Document{},
			page: &Page{AssetID: ""},
			want: ErrEmptyAssetID,
		},
		{
			name: "asset not found",
			doc:  &Document{Assets: map[string]*Asset{"item/image/p-002.jpg": {ID: "p-002", Open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("x")), nil }}}},
			page: &Page{AssetID: "p-001"},
			want: ErrAssetNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.doc.GetAssetByPage(tt.page)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got: %v", tt.want, err)
			}
		})
	}
}

func TestDocumentResolveSpineAssets(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		doc := &Document{}
		pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7Zk7kAAAAASUVORK5CYII=")
		if err != nil {
			t.Fatalf("decode png fixture: %v", err)
		}
		if _, _, err := doc.AddPageWithAsset(bytes.NewReader(pngBytes), int64(len(pngBytes)), "none"); err != nil {
			t.Fatalf("AddPageWithAsset returned error: %v", err)
		}

		if err := doc.ResolveSpineAssets(); err != nil {
			t.Fatalf("ResolveSpineAssets returned error: %v", err)
		}
	})

	t.Run("invalid reference", func(t *testing.T) {
		doc := &Document{
			Pages: []*Page{{Order: 0, AssetID: "p-001"}},
			Assets: map[string]*Asset{
				"item/image/p-002.jpg": {
					ID:       "p-002",
					MimeType: "image/jpeg",
				},
			},
		}

		err := doc.ResolveSpineAssets()
		if !errors.Is(err, ErrAssetNotFound) {
			t.Fatalf("expected ErrAssetNotFound, got: %v", err)
		}
	})
}
