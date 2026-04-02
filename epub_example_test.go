// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"bytes"
	"encoding/base64"
	"fmt"
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
	fmt.Println(page.AssetID == asset.ID)
	fmt.Println(len(doc.Pages), len(doc.Assets))
	// Output:
	// 0
	// true
	// 1 1
}
