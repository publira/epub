// SPDX-License-Identifier: Apache-2.0

// Package kadokawa provides an [epub.Validator] that enforces KADOKAWA
// digital-publishing guidelines.
//
// Checks performed:
//   - Directory layout: assets must reside under item/xhtml/, item/image/,
//     item/style/, or directly under item/.
//   - Image naming: must match KADOKAWA spec §2 (ASCII lowercase alphanumeric,
//     hyphen, underscore; starts with a letter).
//   - Image file size: must be under 4 MB.
//   - Image pixel count: must be under 4,000,000.
//   - Progressive JPEG: not allowed.
//   - XHTML file size: warns when exceeding 256 KB.
package kadokawa

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"path"
	"regexp"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/publira/epub"

	_ "golang.org/x/image/webp"
)

var imageNamePattern = regexp.MustCompile(`^item/image/[a-z][a-z0-9_-]*\.(jpg|jpeg|png|gif)$`)

const (
	maxImagePixelCount = 4000000
	maxImageFileSize   = 4 * 1024 * 1024 // 4 MB
	maxXHTMLFileSize   = 256 * 1024      // 256 KB
)

type validator struct{}

// New returns a Validator that enforces KADOKAWA guidelines.
func New() epub.Validator { return &validator{} }

func (v *validator) ValidateDocument(_ *epub.Document) []error { return nil }

func (v *validator) ValidateAsset(href string, asset *epub.Asset) []error {
	if asset == nil {
		return nil
	}
	var errs []error

	// Directory layout check.
	if err := validateDirectoryRule(href); err != nil {
		errs = append(errs, err)
	}

	mt := strings.ToLower(strings.TrimSpace(asset.MimeType))

	if strings.HasPrefix(mt, "image/") {
		// Image naming.
		if !imageNamePattern.MatchString(href) {
			errs = append(errs, fmt.Errorf("image path %q does not match KADOKAWA naming rules", href))
		}

		// Image file size.
		if asset.Size > uint64(maxImageFileSize) {
			errs = append(errs, &epub.ImageFileSizeTooLargeError{
				Href:   href,
				Limit:  maxImageFileSize,
				Actual: int64(asset.Size),
			})
		}

		// Pixel count and progressive JPEG.
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

func validateDirectoryRule(href string) error {
	dir := path.Dir(href)
	if dir == "." {
		return fmt.Errorf("asset path %q must be under item/xhtml, item/image, or item/style", href)
	}
	if dir == "item" {
		return nil
	}
	if strings.HasPrefix(href, "item/xhtml/") || strings.HasPrefix(href, "item/image/") || strings.HasPrefix(href, "item/style/") {
		return nil
	}
	return fmt.Errorf("asset path %q must be under item/xhtml, item/image, or item/style", href)
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

	var errs []error

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	pixelCount := int64(cfg.Width) * int64(cfg.Height)
	if pixelCount > maxImagePixelCount {
		errs = append(errs, &epub.ImagePixelCountExceededError{
			Href:   href,
			Limit:  maxImagePixelCount,
			Actual: pixelCount,
		})
	}

	if format == "jpeg" && isProgressiveJPEG(data) {
		errs = append(errs, &epub.ProgressiveJPEGNotAllowedError{Href: href})
	}

	return errs
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
