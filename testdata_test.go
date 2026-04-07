// SPDX-License-Identifier: Apache-2.0

package epub_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/publira/epub"
)

// testdataEPUBs returns all .epub paths under testdata/.
// The caller should skip the test when the slice is empty.
func testdataEPUBs(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob("testdata/*.epub")
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestTestdata_DecodeAll(t *testing.T) {
	paths := testdataEPUBs(t)
	if len(paths) == 0 {
		t.Skip("no epub files in testdata/ (see testdata/README.md)")
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			f, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			st, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			doc, err := epub.Decode(f, st.Size())
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if doc.Metadata.Title == "" {
				t.Error("Title is empty")
			}
			if len(doc.Pages) == 0 {
				t.Error("no pages")
			}
		})
	}
}

func TestTestdata_KADOKAWACompliance(t *testing.T) {
	paths := testdataEPUBs(t)
	if len(paths) == 0 {
		t.Skip("no epub files in testdata/ (see testdata/README.md)")
	}
	for _, p := range paths {
		base := filepath.Base(p)
		if !strings.Contains(base, "kadokawa") {
			continue
		}
		t.Run(base, func(t *testing.T) {
			f, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			st, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			_, err = epub.Decode(f, st.Size(), epub.WithCompliance(epub.LevelKADOKAWA))

			// sizecheck EPUBs intentionally contain oversized images;
			// they must be rejected by the compliance check.
			if strings.Contains(base, "sizecheck") {
				if err == nil {
					t.Fatal("expected compliance error for sizecheck EPUB, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("KADOKAWA compliance failed: %v", err)
			}
		})
	}
}

func TestTestdata_DPFJTemplates(t *testing.T) {
	paths := testdataEPUBs(t)
	if len(paths) == 0 {
		t.Skip("no epub files in testdata/ (see testdata/README.md)")
	}
	dpfjPatterns := []string{"book-template", "fixedlayout-template", "dpfj-sample"}
	for _, p := range paths {
		base := filepath.Base(p)
		isDPFJ := false
		for _, pat := range dpfjPatterns {
			if strings.HasPrefix(base, pat) {
				isDPFJ = true
				break
			}
		}
		if !isDPFJ {
			continue
		}
		t.Run(base, func(t *testing.T) {
			f, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			st, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			doc, err := epub.Decode(f, st.Size())
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if doc.Metadata.Title == "" {
				t.Error("Title is empty")
			}
			if doc.Direction != "rtl" {
				t.Errorf("Direction = %q, want rtl", doc.Direction)
			}
		})
	}
}

func TestTestdata_CoverImageRoundTrip(t *testing.T) {
	paths := testdataEPUBs(t)
	if len(paths) == 0 {
		t.Skip("no epub files in testdata/ (see testdata/README.md)")
	}
	for _, p := range paths {
		base := filepath.Base(p)
		// Skip sizecheck EPUBs; they are intentionally non-compliant.
		if strings.Contains(base, "sizecheck") {
			continue
		}
		t.Run(base, func(t *testing.T) {
			f, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			st, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}

			doc, err := epub.Decode(f, st.Size())
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			coverID := doc.Metadata.CoverAssetID
			if coverID == "" {
				t.Log("no cover-image detected; skipping round-trip check")
				return
			}

			// Verify the cover asset actually exists.
			found := false
			for _, a := range doc.Assets {
				if a != nil && a.ID == coverID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("CoverAssetID %q not found in assets", coverID)
			}

			// Encode back and decode; CoverAssetID must survive.
			var buf bytes.Buffer
			if err := epub.Encode(&buf, doc); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			doc2, err := epub.Decode(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				t.Fatalf("re-Decode failed: %v", err)
			}
			if doc2.Metadata.CoverAssetID != coverID {
				t.Errorf("CoverAssetID after round-trip = %q, want %q", doc2.Metadata.CoverAssetID, coverID)
			}
		})
	}
}
