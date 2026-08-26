// SPDX-License-Identifier: Apache-2.0

package kadokawa_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/publira/epub"
	"github.com/publira/epub/profile/kadokawa"
)

func testPNGBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestValidateAsset_DirectoryLayoutError(t *testing.T) {
	v := kadokawa.New()
	asset := &epub.Asset{ID: "x", MimeType: "image/jpeg", Size: 100}
	errs := v.ValidateAsset("bad/path.jpg", asset)
	if len(errs) == 0 {
		t.Fatal("expected directory-layout error")
	}
}

func TestValidateAsset_ImageNamingError(t *testing.T) {
	v := kadokawa.New()
	pngData := testPNGBytes()
	asset := &epub.Asset{
		ID: "x", MimeType: "image/png", Size: uint64(len(pngData)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(pngData)), nil
		},
	}
	errs := v.ValidateAsset("item/image/BADNAME.png", asset)
	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "KADOKAWA") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected naming error, got: %v", errs)
	}
}

func TestValidateAsset_ValidImage(t *testing.T) {
	v := kadokawa.New()
	pngData := testPNGBytes()
	asset := &epub.Asset{
		ID: "p-001", MimeType: "image/png", Size: uint64(len(pngData)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(pngData)), nil
		},
	}
	errs := v.ValidateAsset("item/image/p-001.png", asset)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid image, got: %v", errs)
	}
}

func TestValidateAsset_XHTMLSizeWarning(t *testing.T) {
	v := kadokawa.New()
	asset := &epub.Asset{
		ID: "x-001", MimeType: "application/xhtml+xml", Size: 300 * 1024,
	}
	errs := v.ValidateAsset("item/xhtml/page.xhtml", asset)
	found := false
	for _, err := range errs {
		var w *epub.ValidationWarningError
		if errors.As(err, &w) && strings.Contains(w.Message, "XHTML") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected XHTML warning, got: %v", errs)
	}
}

func TestValidateDocument_ReturnsNil(t *testing.T) {
	v := kadokawa.New()
	errs := v.ValidateDocument(&epub.Document{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}
