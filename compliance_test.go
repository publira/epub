// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"errors"
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
	var missing *ErrManifestPhysicalMissing
	if !errors.As(err, &missing) {
		t.Fatalf("expected ErrManifestPhysicalMissing, got: %v", err)
	}
	if missing.Href != "item/image/p-001.jpg" {
		t.Fatalf("unexpected missing href: %s", missing.Href)
	}
}
