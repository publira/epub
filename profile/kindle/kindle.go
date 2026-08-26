// SPDX-License-Identifier: Apache-2.0

// Package kindle provides an [epub.Validator] that enforces Amazon Kindle
// (KDP) publishing guidelines.
//
// All violations are returned as [epub.ValidationWarningError] so they appear as
// non-fatal warnings during Decode.
//
// Checks performed:
//   - Image file size: warns when exceeding 5 MB.
//   - Progressive JPEG: warns (not supported by Kindle).
//   - XHTML file size: warns when exceeding 256 KB.
package kindle

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/publira/epub"

	_ "golang.org/x/image/webp"
)

const (
	maxImageFileSizeKindle = 5 * 1024 * 1024 // 5 MB
	maxXHTMLFileSize       = 256 * 1024      // 256 KB
)

type validator struct{}

// New returns a Validator that enforces Kindle (KDP) guidelines.
func New() epub.Validator { return &validator{} }

func (v *validator) ValidateDocument(_ *epub.Document) []error { return nil }

func (v *validator) ValidateAsset(href string, asset *epub.Asset) []error {
	if asset == nil {
		return nil
	}
	var errs []error

	mt := strings.ToLower(strings.TrimSpace(asset.MimeType))

	if strings.HasPrefix(mt, "image/") {
		// Image file size (5 MB Kindle limit).
		if asset.Size > uint64(maxImageFileSizeKindle) {
			errs = append(errs, &epub.ValidationWarningError{
				Message: fmt.Sprintf("image %q file size %s exceeds Kindle %s limit",
					href, formatBytes(int64(asset.Size)), formatBytes(maxImageFileSizeKindle)),
			})
		}

		// Progressive JPEG.
		if asset.Open != nil {
			errs = append(errs, validateImageSpecs(href, asset)...)
		}
	} else if strings.Contains(mt, "xhtml") {
		if asset.Size > uint64(maxXHTMLFileSize) {
			errs = append(errs, &epub.ValidationWarningError{
				Message: fmt.Sprintf("XHTML file %s exceeds 256KB (%s), may cause RS performance issues",
					href, formatBytes(int64(asset.Size))),
			})
		}
	}

	return errs
}

func validateImageSpecs(href string, asset *epub.Asset) []error {
	rc, err := asset.Open()
	if err != nil {
		return nil
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}

	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	if format == "jpeg" && isProgressiveJPEG(data) {
		return []error{&epub.ValidationWarningError{
			Message: fmt.Sprintf("image %q is a progressive JPEG, which is not supported by Kindle", href),
		}}
	}

	return nil
}

func isProgressiveJPEG(data []byte) bool {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0xFF && data[i+1] == 0xC2 {
			return true
		}
	}
	return false
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	} else if b < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
}
