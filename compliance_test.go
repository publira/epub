// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestValidateCompliance_FlexibleAllows(t *testing.T) {
	manifest := map[string]manifestItem{
		"weird/path/foo.jpg": {Href: "weird/path/foo.jpg", MediaType: "image/jpeg"},
	}
	files := map[string]struct{}{
		"weird/path/foo.jpg": {},
	}
	filesByName := make(map[string]*zip.File)
	_, err := validateCompliance(LevelFlexible, files, manifest, filesByName)
	if err != nil {
		t.Fatalf("expected nil in flexible mode, got: %v", err)
	}
}

func TestValidateCompliance_EBPAJDirectoryViolation(t *testing.T) {
	manifest := map[string]manifestItem{
		"foo/bar.jpg": {Href: "foo/bar.jpg", MediaType: "image/jpeg"},
	}
	files := map[string]struct{}{
		"foo/bar.jpg": {},
	}
	filesByName := make(map[string]*zip.File)
	_, err := validateCompliance(LevelEBPAJ, files, manifest, filesByName)
	if err == nil {
		t.Fatal("expected directory-layout error")
	}
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected DecodeError, got: %T", err)
	}
	if de.Rule != "directory-layout" {
		t.Fatalf("unexpected rule: %s", de.Rule)
	}
}

func TestValidateCompliance_MissingPhysicalFile(t *testing.T) {
	manifest := map[string]manifestItem{
		"item/image/p-001.jpg": {Href: "item/image/p-001.jpg", MediaType: "image/jpeg"},
	}
	files := map[string]struct{}{}
	filesByName := make(map[string]*zip.File)
	_, err := validateCompliance(LevelKADOKAWA, files, manifest, filesByName)
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

func TestPreflightEncode_FlexibleReturnsNil(t *testing.T) {
	doc := &Document{Assets: map[string]*Asset{
		"baddir/foo.jpg": {ID: "x", MimeType: "image/jpeg", Size: 100},
	}}
	warnings := preflightEncode(LevelFlexible, doc)
	if warnings != nil {
		t.Fatalf("expected nil, got: %v", warnings)
	}
}

func TestPreflightEncode_NilDocReturnsNil(t *testing.T) {
	warnings := preflightEncode(LevelKADOKAWA, nil)
	if warnings != nil {
		t.Fatalf("expected nil for nil doc, got: %v", warnings)
	}
}

func TestPreflightEncode_ImageFileSizeWarning(t *testing.T) {
	doc := &Document{Assets: map[string]*Asset{
		"item/image/p-001.jpg": {
			ID: "p-001", MimeType: "image/jpeg", Size: 5 * 1024 * 1024,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
			},
		},
	}}
	warnings := preflightEncode(LevelEBPAJ, doc)
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

func TestPreflightEncode_ImageNamingWarning(t *testing.T) {
	doc := &Document{Assets: map[string]*Asset{
		"item/image/BADNAME.jpg": {
			ID: "p-001", MimeType: "image/jpeg", Size: 100,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
			},
		},
	}}
	for _, level := range []ComplianceLevel{LevelEBPAJ, LevelKADOKAWA} {
		warnings := preflightEncode(level, doc)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "naming pattern") {
				found = true
			}
		}
		if !found {
			t.Fatalf("level %d: expected naming warning, got: %v", level, warnings)
		}
	}
}

func TestPreflightEncode_DirectoryLayoutWarning(t *testing.T) {
	doc := &Document{Assets: map[string]*Asset{
		"wrong/place.jpg": {
			ID: "p-001", MimeType: "image/jpeg", Size: 100,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
			},
		},
	}}
	warnings := preflightEncode(LevelKADOKAWA, doc)
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

func TestPreflightEncode_XHTMLSizeWarning(t *testing.T) {
	doc := &Document{Assets: map[string]*Asset{
		"item/xhtml/page.xhtml": {
			ID: "x-001", MimeType: "application/xhtml+xml", Size: 300 * 1024,
		},
	}}
	warnings := preflightEncode(LevelEBPAJ, doc)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "XHTML") && strings.Contains(w, "exceeds") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected XHTML size warning, got: %v", warnings)
	}
}

func TestPreflightEncode_ValidDocNoWarnings(t *testing.T) {
	png := testPNGBytes()
	doc := &Document{Assets: map[string]*Asset{
		"item/image/p-001.png": {
			ID: "p-001", MimeType: "image/png", Size: uint64(len(png)),
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(png)), nil
			},
		},
	}}
	warnings := preflightEncode(LevelKADOKAWA, doc)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for valid doc, got: %v", warnings)
	}
}

// --- LevelKindle tests ---

func TestValidateCompliance_KindleAllowsFlexibleLayout(t *testing.T) {
	// Kindle does not enforce EBPAJ/KADOKAWA directory or naming rules.
	manifest := map[string]manifestItem{
		"images/cover.jpg": {Href: "images/cover.jpg", MediaType: "image/jpeg"},
	}
	files := map[string]struct{}{
		"images/cover.jpg": {},
	}
	filesByName := make(map[string]*zip.File)
	_, err := validateCompliance(LevelKindle, files, manifest, filesByName)
	if err != nil {
		t.Fatalf("LevelKindle should not enforce directory layout, got: %v", err)
	}
}

func TestValidateCompliance_KindleImageSizeWarning(t *testing.T) {
	// Create a ZIP with an image exceeding 5MB.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	bigData := make([]byte, 6*1024*1024) // 6MB
	// Write minimal valid PNG header so image.DecodeConfig doesn't fail
	copy(bigData, testPNGBytes())
	f, _ := w.Create("images/big.png")
	_, _ = f.Write(bigData)
	_ = w.Close()

	r, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	filesByName := make(map[string]*zip.File)
	for _, zf := range r.File {
		filesByName[zf.Name] = zf
	}

	manifest := map[string]manifestItem{
		"images/big.png": {Href: "images/big.png", MediaType: "image/png"},
	}
	filesSet := map[string]struct{}{
		"images/big.png": {},
	}

	warnings, err := validateCompliance(LevelKindle, filesSet, manifest, filesByName)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	found := false
	for _, w := range *warnings {
		if strings.Contains(w, "Kindle") && strings.Contains(w, "exceeds") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Kindle image size warning, got: %v", *warnings)
	}
}

// testProgressiveJPEGBytes returns minimal bytes that look like a progressive JPEG
// (SOI + SOF2 marker) for testing purposes.
func testProgressiveJPEGBytes() []byte {
	// Minimal progressive JPEG:
	// SOI (0xFFD8) + APP0 (skip) + SOF2 marker
	// We build a realistic-enough stream so isProgressiveJPEG detects it.
	return []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xE0, // APP0
		0x00, 0x10, // length 16
		0x4A, 0x46, 0x49, 0x46, 0x00, // "JFIF\0"
		0x01, 0x01, // version
		0x00,       // units
		0x00, 0x01, // Xdensity
		0x00, 0x01, // Ydensity
		0x00, 0x00, // thumbnail
		0xFF, 0xC2, // SOF2 (progressive)
		0x00, 0x0B, // length 11
		0x08,       // precision
		0x00, 0x01, // height=1
		0x00, 0x01, // width=1
		0x01,       // components=1
		0x01,       // component id
		0x11,       // sampling
		0x00,       // quant table
		0xFF, 0xD9, // EOI
	}
}

func TestValidateCompliance_KindleProgressiveJPEGWarning(t *testing.T) {
	jpegData := testProgressiveJPEGBytes()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("images/photo.jpg")
	_, _ = f.Write(jpegData)
	_ = w.Close()

	r, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	filesByName := make(map[string]*zip.File)
	for _, zf := range r.File {
		filesByName[zf.Name] = zf
	}

	manifest := map[string]manifestItem{
		"images/photo.jpg": {Href: "images/photo.jpg", MediaType: "image/jpeg"},
	}
	filesSet := map[string]struct{}{
		"images/photo.jpg": {},
	}

	warnings, err := validateCompliance(LevelKindle, filesSet, manifest, filesByName)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	found := false
	for _, w := range *warnings {
		if strings.Contains(w, "progressive JPEG") && strings.Contains(w, "Kindle") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Kindle progressive JPEG warning, got: %v", *warnings)
	}
}

func TestValidateCompliance_KindleNoWarningForSmallImage(t *testing.T) {
	png := testPNGBytes()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("images/small.png")
	_, _ = f.Write(png)
	_ = w.Close()

	r, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	filesByName := make(map[string]*zip.File)
	for _, zf := range r.File {
		filesByName[zf.Name] = zf
	}

	manifest := map[string]manifestItem{
		"images/small.png": {Href: "images/small.png", MediaType: "image/png"},
	}
	filesSet := map[string]struct{}{
		"images/small.png": {},
	}

	warnings, err := validateCompliance(LevelKindle, filesSet, manifest, filesByName)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(*warnings) != 0 {
		t.Fatalf("expected no warnings for small valid image, got: %v", *warnings)
	}
}

func TestPreflightEncode_KindleImageFileSizeWarning(t *testing.T) {
	doc := &Document{Assets: map[string]*Asset{
		"images/big.jpg": {
			ID: "big", MimeType: "image/jpeg", Size: 6 * 1024 * 1024,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
			},
		},
	}}
	warnings := preflightEncode(LevelKindle, doc)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "Kindle") && strings.Contains(w, "exceeds") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Kindle file size warning, got: %v", warnings)
	}
}

func TestPreflightEncode_KindleProgressiveJPEGWarning(t *testing.T) {
	jpegData := testProgressiveJPEGBytes()
	doc := &Document{Assets: map[string]*Asset{
		"images/photo.jpg": {
			ID: "photo", MimeType: "image/jpeg", Size: uint64(len(jpegData)),
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(jpegData)), nil
			},
		},
	}}
	warnings := preflightEncode(LevelKindle, doc)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "progressive JPEG") && strings.Contains(w, "Kindle") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Kindle progressive JPEG warning, got: %v", warnings)
	}
}

func TestPreflightEncode_KindleNoDirectoryWarning(t *testing.T) {
	png := testPNGBytes()
	doc := &Document{Assets: map[string]*Asset{
		"custom/path/image.png": {
			ID: "img", MimeType: "image/png", Size: uint64(len(png)),
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(png)), nil
			},
		},
	}}
	warnings := preflightEncode(LevelKindle, doc)
	for _, w := range warnings {
		if strings.Contains(w, "directory layout") {
			t.Fatalf("LevelKindle should not warn about directory layout, got: %v", w)
		}
	}
}

func TestPreflightEncode_KindleNoNamingWarning(t *testing.T) {
	png := testPNGBytes()
	doc := &Document{Assets: map[string]*Asset{
		"images/MyImage_UPPERCASE.png": {
			ID: "img", MimeType: "image/png", Size: uint64(len(png)),
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(png)), nil
			},
		},
	}}
	warnings := preflightEncode(LevelKindle, doc)
	for _, w := range warnings {
		if strings.Contains(w, "naming pattern") {
			t.Fatalf("LevelKindle should not warn about naming, got: %v", w)
		}
	}
}

func TestPreflightEncode_KindleValidDocNoWarnings(t *testing.T) {
	png := testPNGBytes()
	doc := &Document{Assets: map[string]*Asset{
		"images/cover.png": {
			ID: "cover", MimeType: "image/png", Size: uint64(len(png)),
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(png)), nil
			},
		},
	}}
	warnings := preflightEncode(LevelKindle, doc)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for valid Kindle doc, got: %v", warnings)
	}
}

func TestPreflightEncode_Kindle4MBImageNoWarning(t *testing.T) {
	// 4.5MB is under the Kindle 5MB limit but over the EBPAJ/KADOKAWA 4MB limit.
	doc := &Document{Assets: map[string]*Asset{
		"images/large.jpg": {
			ID: "large", MimeType: "image/jpeg", Size: uint64(4.5 * 1024 * 1024),
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader([]byte("fake"))), nil
			},
		},
	}}
	warnings := preflightEncode(LevelKindle, doc)
	for _, w := range warnings {
		if strings.Contains(w, "file size") && strings.Contains(w, "exceeds") {
			t.Fatalf("4.5MB should not trigger Kindle 5MB limit warning, got: %v", w)
		}
	}

	// Same size should trigger EBPAJ warning.
	warningsEBPAJ := preflightEncode(LevelEBPAJ, doc)
	found := false
	for _, w := range warningsEBPAJ {
		if strings.Contains(w, "file size") && strings.Contains(w, "exceeds") {
			found = true
		}
	}
	if !found {
		t.Fatalf("4.5MB should trigger EBPAJ 4MB limit warning, got: %v", warningsEBPAJ)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{500, "500B"},
		{1024, "1.0KB"},
		{5 * 1024 * 1024, "5.0MB"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			got := formatBytes(tt.input)
			if got != tt.want {
				t.Fatalf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
