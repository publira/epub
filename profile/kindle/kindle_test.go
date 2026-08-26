// SPDX-License-Identifier: Apache-2.0

package kindle_test

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
	"github.com/publira/epub/profile/kindle"
)

func testPNGBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// testProgressiveJPEGBytes returns minimal bytes that look like a progressive JPEG.
func testProgressiveJPEGBytes() []byte {
	return []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xE0, // APP0
		0x00, 0x10, // length 16
		0x4A, 0x46, 0x49, 0x46, 0x00, // "JFIF\0"
		0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
		0xFF, 0xC2, // SOF2 (progressive)
		0x00, 0x0B, 0x08, 0x00, 0x01, 0x00, 0x01, 0x01, 0x01, 0x11, 0x00,
		0xFF, 0xD9, // EOI
	}
}

func TestValidateAsset_ImageFileSizeWarning(t *testing.T) {
	v := kindle.New()
	asset := &epub.Asset{
		ID: "big", MimeType: "image/jpeg", Size: 6 * 1024 * 1024,
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
		},
	}
	errs := v.ValidateAsset("images/big.jpg", asset)
	found := false
	for _, err := range errs {
		var w *epub.ValidationWarningError
		if errors.As(err, &w) && strings.Contains(w.Message, "Kindle") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Kindle file size warning, got: %v", errs)
	}
}

func TestValidateAsset_ProgressiveJPEGWarning(t *testing.T) {
	v := kindle.New()
	jpegData := testProgressiveJPEGBytes()
	asset := &epub.Asset{
		ID: "photo", MimeType: "image/jpeg", Size: uint64(len(jpegData)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(jpegData)), nil
		},
	}
	errs := v.ValidateAsset("images/photo.jpg", asset)
	found := false
	for _, err := range errs {
		var w *epub.ValidationWarningError
		if errors.As(err, &w) && strings.Contains(w.Message, "progressive JPEG") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected progressive JPEG warning, got: %v", errs)
	}
}

func TestValidateAsset_ValidSmallImage(t *testing.T) {
	v := kindle.New()
	pngData := testPNGBytes()
	asset := &epub.Asset{
		ID: "img", MimeType: "image/png", Size: uint64(len(pngData)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(pngData)), nil
		},
	}
	errs := v.ValidateAsset("images/small.png", asset)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for small valid image, got: %v", errs)
	}
}

func TestValidateAsset_NoDirectoryLayoutEnforcement(t *testing.T) {
	v := kindle.New()
	pngData := testPNGBytes()
	asset := &epub.Asset{
		ID: "img", MimeType: "image/png", Size: uint64(len(pngData)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(pngData)), nil
		},
	}
	// Kindle should not enforce directory layout.
	errs := v.ValidateAsset("custom/path/image.png", asset)
	if len(errs) != 0 {
		t.Fatalf("Kindle should not enforce directory layout, got: %v", errs)
	}
}

func TestValidateAsset_XHTMLSizeWarning(t *testing.T) {
	v := kindle.New()
	asset := &epub.Asset{
		ID: "x-001", MimeType: "application/xhtml+xml", Size: 300 * 1024,
	}
	errs := v.ValidateAsset("content/page.xhtml", asset)
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
	v := kindle.New()
	errs := v.ValidateDocument(&epub.Document{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}
