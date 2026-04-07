// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
)

func ExampleDecode_structuredErrors() {
	epubData := minimalEPUBMissingAssetExample()

	_, err := Decode(bytes.NewReader(epubData), int64(len(epubData)))
	if err == nil {
		fmt.Println("ok")
		return
	}

	var missing *ManifestPhysicalMissingError
	if errors.As(err, &missing) {
		fmt.Println("missing", missing.Href)
	}

	// Output:
	// missing item/image/p-001.jpg
}

func minimalEPUBMissingAssetExample() []byte {
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

	w, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		panic(err)
	}
	if _, err := w.Write([]byte(mimeTypeValue)); err != nil {
		panic(err)
	}
	for _, file := range []struct {
		name string
		body string
	}{
		{name: "META-INF/container.xml", body: container},
		{name: "item/standard.opf", body: opf},
		{name: "item/xhtml/p-001.xhtml", body: xhtml},
	} {
		w, err := zw.Create(file.name)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write([]byte(file.body)); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
