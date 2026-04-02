// SPDX-License-Identifier: Apache-2.0

package epub_test

import (
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
		if !strings.Contains(filepath.Base(p), "kadokawa") {
			continue
		}
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

			_, err = epub.Decode(f, st.Size(), epub.WithCompliance(epub.LevelKADOKAWA))
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
