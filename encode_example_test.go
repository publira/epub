// SPDX-License-Identifier: Apache-2.0

package epub_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/publira/epub"
	"github.com/publira/epub/profile/kadokawa"
	"github.com/publira/epub/profile/kindle"
)

func ExampleEncode_preflightWithProfile() {
	doc := &epub.Document{
		Metadata:  epub.Metadata{Title: "My Book"},
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

func ExampleEncode_preflightWithMultipleProfiles() {
	doc := &epub.Document{
		Metadata:  epub.Metadata{Title: "Multi-Profile Book"},
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
		epub.WithValidator(kadokawa.New(), kindle.New()),
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
