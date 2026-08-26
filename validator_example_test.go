// SPDX-License-Identifier: Apache-2.0

package epub_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/publira/epub"
	"github.com/publira/epub/profile/kadokawa"
	"github.com/publira/epub/profile/kindle"
)

func ExampleWithValidator() {
	epubData := minimalValidEPUB()

	doc, err := epub.Decode(
		bytes.NewReader(epubData), int64(len(epubData)),
		epub.WithValidator(kadokawa.New(), kindle.New()),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(doc.Metadata.Title)
	fmt.Println(len(doc.Pages), "page(s)")
	// Output:
	// Test Book
	// 1 page(s)
}

func ExampleWithValidator_warnings() {
	epubData := minimalValidEPUB()

	// Kindle validator returns only warnings, never fatal errors.
	doc, err := epub.Decode(
		bytes.NewReader(epubData), int64(len(epubData)),
		epub.WithValidator(kindle.New()),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	if len(doc.Warnings) == 0 {
		fmt.Println("no warnings")
	} else {
		for _, w := range doc.Warnings {
			fmt.Println("warning:", w)
		}
	}
	// Output:
	// no warnings
}

func ExampleWithValidator_encode() {
	doc := &epub.Document{
		Metadata:  epub.Metadata{Title: "Demo"},
		Direction: "rtl",
		Layout:    epub.LayoutPrePaginated,
		Assets: map[string]*epub.Asset{
			"item/image/p-001.jpg": {
				ID:       "p-001",
				MimeType: "image/jpeg",
				Size:     100,
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("fake")), nil
				},
			},
		},
		Pages: []*epub.Page{{Order: 0, AssetID: "p-001", Spread: "right"}},
	}

	var warnings []string
	var out bytes.Buffer
	err := epub.Encode(&out, doc,
		epub.WithValidator(kadokawa.New()),
		epub.WithEncodeWarningCollector(func(w string) { warnings = append(warnings, w) }),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(len(warnings), "warning(s)")
	// Output:
	// 0 warning(s)
}

// minimalValidEPUB builds a small in-memory EPUB that passes KADOKAWA validation.
func minimalValidEPUB() []byte {
	container := `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="item/standard.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
  <manifest>
    <item id="xhtml-1" href="xhtml/p-001.xhtml" media-type="application/xhtml+xml"/>
    <item id="p-001" href="image/p-001.jpg" media-type="image/jpeg"/>
  </manifest>
  <spine page-progression-direction="rtl">
    <itemref idref="xhtml-1" properties="page-spread-right"/>
  </spine>
</package>`

	xhtml := `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>p1</title></head><body><p>hello</p></body></html>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	_, _ = w.Write([]byte("application/epub+zip"))

	for _, f := range []struct {
		name, body string
	}{
		{"META-INF/container.xml", container},
		{"item/standard.opf", opf},
		{"item/xhtml/p-001.xhtml", xhtml},
		{"item/image/p-001.jpg", "fake-jpeg"},
	} {
		w, _ := zw.Create(f.name)
		_, _ = w.Write([]byte(f.body))
	}
	_ = zw.Close()
	return buf.Bytes()
}
