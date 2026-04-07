// SPDX-License-Identifier: Apache-2.0

package ebpaj_test

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
	"github.com/publira/epub/profile/ebpaj"
)

func testPNGBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestValidateAsset_DirectoryLayoutError(t *testing.T) {
	v := ebpaj.New()
	asset := &epub.Asset{ID: "x", MimeType: "image/jpeg", Size: 100}
	errs := v.ValidateAsset("bad/path.jpg", asset)
	if len(errs) == 0 {
		t.Fatal("expected directory-layout error")
	}
	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), "item/xhtml") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected directory layout error, got: %v", errs)
	}
}

func TestValidateAsset_ImageNamingError(t *testing.T) {
	v := ebpaj.New()
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
		if strings.Contains(err.Error(), "item/image/p-000") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected naming error, got: %v", errs)
	}
}

func TestValidateAsset_ValidImage(t *testing.T) {
	v := ebpaj.New()
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

func TestValidateAsset_ImageFileSizeError(t *testing.T) {
	v := ebpaj.New()
	asset := &epub.Asset{
		ID: "p-001", MimeType: "image/jpeg", Size: 5 * 1024 * 1024,
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
		},
	}
	errs := v.ValidateAsset("item/image/p-001.jpg", asset)
	found := false
	for _, err := range errs {
		var fsErr *epub.ImageFileSizeTooLargeError
		if errors.As(err, &fsErr) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ImageFileSizeTooLargeError, got: %v", errs)
	}
}

func TestValidateAsset_XHTMLSizeWarning(t *testing.T) {
	v := ebpaj.New()
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
	v := ebpaj.New()
	errs := v.ValidateDocument(&epub.Document{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidateAsset_NilAsset(t *testing.T) {
	v := ebpaj.New()
	errs := v.ValidateAsset("item/image/p-001.png", nil)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for nil asset, got: %v", errs)
	}
}
