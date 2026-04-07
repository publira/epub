// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"bytes"
	"errors"
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
