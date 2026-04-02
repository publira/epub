// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"bytes"
	"encoding/base64"
	"errors"
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

func TestAddPageWithAssetSharesID(t *testing.T) {
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
	if page.AssetID != asset.ID {
		t.Fatalf("page asset id (%s) and asset id (%s) must match", page.AssetID, asset.ID)
	}
	if page.Spread != "left" {
		t.Fatalf("unexpected spread: %s", page.Spread)
	}
	if page.Width != 1 || page.Height != 1 {
		t.Fatalf("unexpected viewport: %dx%d", page.Width, page.Height)
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
