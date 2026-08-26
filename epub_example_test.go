// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func ExampleDocument_AddAsset() {
	doc := &Document{}
	img := strings.NewReader("fake-image-bytes")

	href, asset, err := doc.AddAsset("image/jpeg", img, int64(img.Len()))
	if err != nil {
		fmt.Println("error")
		return
	}

	fmt.Println(href)
	fmt.Println(asset.ID)
	// Output:
	// item/image/p-001.jpg
	// p-001
}

func ExampleDocument_AddPageWithAsset() {
	doc := &Document{
		Metadata:  Metadata{Title: "Demo"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
	}
	pngBytes, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7Zk7kAAAAASUVORK5CYII=")
	img := bytes.NewReader(pngBytes)

	page, asset, err := doc.AddPageWithAsset(img, int64(img.Len()), "right")
	if err != nil {
		fmt.Println("error")
		return
	}

	fmt.Println(page.Order)
	fmt.Println(strings.HasPrefix(page.AssetID, "xhtml-"))
	fmt.Println(page.Href)
	fmt.Println(asset.MimeType)
	fmt.Println(len(doc.Pages), len(doc.Assets))
	// Output:
	// 0
	// true
	// item/xhtml/p-001.xhtml
	// image/png
	// 1 2
}

// customValidator is a minimal Validator that rejects images exceeding a
// configurable byte-size limit.
type customValidator struct {
	maxImageSize uint64
}

func (v *customValidator) ValidateDocument(_ *Document) []error { return nil }

func (v *customValidator) ValidateAsset(href string, asset *Asset) []error {
	if asset == nil {
		return nil
	}
	if strings.HasPrefix(asset.MimeType, "image/") && asset.Size > v.maxImageSize {
		return []error{&ValidationWarningError{
			Message: fmt.Sprintf("image %q is too large: %d bytes", href, asset.Size),
		}}
	}
	return nil
}

func ExampleDecode_withCustomValidator() {
	epubData := exampleMinimalEPUB()

	v := &customValidator{maxImageSize: 1024 * 1024} // 1 MB

	doc, err := Decode(
		bytes.NewReader(epubData), int64(len(epubData)),
		WithValidator(v),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(doc.Metadata.Title)
	fmt.Println(len(doc.Warnings), "warning(s)")
	// Output:
	// Test Book
	// 0 warning(s)
}

func ExampleDecode_withMultipleValidators() {
	epubData := exampleMinimalEPUB()

	v1 := &customValidator{maxImageSize: 1024 * 1024} // 1 MB
	v2 := &customValidator{maxImageSize: 512 * 1024}  // 512 KB

	doc, err := Decode(
		bytes.NewReader(epubData), int64(len(epubData)),
		WithValidator(v1, v2),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(doc.Metadata.Title)
	fmt.Println(len(doc.Warnings), "warning(s)")
	// Output:
	// Test Book
	// 0 warning(s)
}

func ExampleEncode_withPreflightValidator() {
	doc := &Document{
		Metadata:  Metadata{Title: "Demo"},
		Direction: "rtl",
		Layout:    LayoutPrePaginated,
		Assets: map[string]*Asset{
			"item/image/p-001.jpg": {
				ID:       "p-001",
				MimeType: "image/jpeg",
				Size:     100,
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("fake")), nil
				},
			},
		},
		Pages: []*Page{{Order: 0, AssetID: "p-001", Spread: "right"}},
	}

	v := &customValidator{maxImageSize: 50} // intentionally small

	var warnings []string
	var out bytes.Buffer
	err := Encode(&out, doc,
		WithValidator(v),
		WithEncodeWarningCollector(func(w string) { warnings = append(warnings, w) }),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(len(warnings), "warning(s)")
	for _, w := range warnings {
		fmt.Println(w)
	}
	// Output:
	// 1 warning(s)
	// image "item/image/p-001.jpg" is too large: 100 bytes
}

func exampleMinimalEPUB() []byte {
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
	_, _ = w.Write([]byte(mimeTypeValue))
	for _, f := range []struct{ name, body string }{
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
