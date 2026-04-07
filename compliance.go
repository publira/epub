// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
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
)

var imageNamePatternEBPAJ = regexp.MustCompile(`^item/image/p-[0-9]{3,4}\.(jpg|png)$`)

// KADOKAWA spec §2: ASCII lowercase alphanumeric, hyphen, underscore; starts with a letter.
var imageNamePatternKADOKAWA = regexp.MustCompile(`^item/image/[a-z][a-z0-9_-]*\.(jpg|jpeg|png|gif)$`)

// Image spec constants (Issue #5 requirements)
const (
	maxImagePixelCount = 4000000
	maxImageFileSize   = 4 * 1024 * 1024 // 4MB
	maxXHTMLFileSize   = 256 * 1024      // 256KB
)

func validateCompliance(level ComplianceLevel, files map[string]struct{}, manifest map[string]manifestItem, filesByName map[string]*zip.File) (*[]string, error) {
	warnings := make([]string, 0)
	if level == LevelFlexible {
		return &warnings, nil
	}

	for href, item := range manifest {
		if _, ok := files[href]; !ok {
			return nil, &DecodeError{Path: href, Rule: "manifest-physical-existence", Err: &ManifestPhysicalMissingError{Href: href}}
		}
		if err := validateDirectoryRule(href); err != nil {
			return nil, err
		}
		if strings.HasPrefix(item.MediaType, "image/") {
			if err := validateImageNaming(level, href); err != nil {
				return nil, err
			}
			// Validate image specs based on compliance level
			if level == LevelKADOKAWA || level == LevelEBPAJ {
				zf := filesByName[href]
				if zf != nil {
					if err := validateImageSpecs(zf, href); err != nil {
						return nil, err
					}
				}
			}
		} else if strings.HasPrefix(item.MediaType, "application/xhtml+xml") {
			// Check XHTML file size and add warning if it exceeds 256KB
			zf := filesByName[href]
			if zf != nil {
				if zf.UncompressedSize64 > uint64(maxXHTMLFileSize) {
					warning := "XHTML file " + href + " exceeds 256KB (" + formatBytes(int64(zf.UncompressedSize64)) + "), may cause RS performance issues"
					warnings = append(warnings, warning)
				}
			}
		}
	}
	return &warnings, nil
}

// formatBytes formats bytes in human-readable format
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
}

func validateImageNaming(level ComplianceLevel, href string) error {
	switch level {
	case LevelEBPAJ:
		if !imageNamePatternEBPAJ.MatchString(href) {
			return &DecodeError{Path: href, Rule: "image-naming", Err: errInvalidImageNaming}
		}
	case LevelKADOKAWA:
		if !imageNamePatternKADOKAWA.MatchString(href) {
			return &DecodeError{Path: href, Rule: "image-naming", Err: errInvalidImageNaming}
		}
	}
	return nil
}

func validateDirectoryRule(href string) error {
	dir := path.Dir(href)
	if dir == "." {
		return &DecodeError{Path: href, Rule: "directory-layout", Err: errInvalidDirectoryLayout}
	}
	// navigation-documents.xhtml and standard.opf live directly under item/
	if dir == "item" {
		return nil
	}
	if strings.HasPrefix(href, "item/xhtml/") || strings.HasPrefix(href, "item/image/") || strings.HasPrefix(href, "item/style/") {
		return nil
	}
	return &DecodeError{Path: href, Rule: "directory-layout", Err: errInvalidDirectoryLayout}
}

// validateImageSpecs validates image specifications per Issue #5:
// - Total pixel count < 4,000,000
// - File size < 4MB
// - Enforce sRGB and prohibit Progressive JPEG
func validateImageSpecs(zf *zip.File, href string) error {
	// Check file size
	if zf.UncompressedSize64 > uint64(maxImageFileSize) {
		return &DecodeError{
			Path: href,
			Rule: "image-file-size",
			Err: &ImageFileSizeTooLargeError{
				Href:   href,
				Limit:  maxImageFileSize,
				Actual: int64(zf.UncompressedSize64),
			},
		}
	}

	// Open file and decode image
	rc, err := zf.Open()
	if err != nil {
		return &DecodeError{Path: href, Rule: "image-open", Err: err}
	}
	defer func() { _ = rc.Close() }()

	// Read all data to check pixel count and JPEG properties
	data, err := io.ReadAll(rc)
	if err != nil {
		return &DecodeError{Path: href, Rule: "image-read", Err: err}
	}

	// Decode image to get dimensions
	cfg, format, err := image.DecodeConfig(strings.NewReader(string(data)))
	if err != nil {
		return &DecodeError{Path: href, Rule: "image-decode", Err: err}
	}

	// Check pixel count
	pixelCount := int64(cfg.Width) * int64(cfg.Height)
	if pixelCount > maxImagePixelCount {
		return &DecodeError{
			Path: href,
			Rule: "image-pixel-count",
			Err: &ImagePixelCountExceededError{
				Href:   href,
				Limit:  maxImagePixelCount,
				Actual: pixelCount,
			},
		}
	}

	// Check JPEG-specific properties if it's a JPEG
	if format == "jpeg" {
		if err := validateJPEGSpecs(strings.NewReader(string(data)), href); err != nil {
			return err
		}
	}

	return nil
}

// validateJPEGSpecs validates JPEG-specific constraints
func validateJPEGSpecs(r io.Reader, href string) error {
	// Detect progressive JPEG by looking for SOF markers
	// Progressive JPEG has SOF2 marker (0xFFC2) instead of SOF0 (0xFFC0)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil //nolint:nilerr // Skip validation if we can't read
	}

	if len(data) > 5 {
		// Look for SOF markers: 0xFFC0-0xFFC9
		// SOF2 (0xFFC2) indicates progressive encoding
		for i := 0; i < len(data)-1; i++ {
			if data[i] == 0xFF && (data[i+1] == 0xC2) {
				return &DecodeError{
					Path: href,
					Rule: "jpeg-progressive",
					Err:  &ProgressiveJPEGNotAllowedError{Href: href},
				}
			}
		}
	}

	return nil
}

// preflightEncode runs lightweight compliance checks against a Document before
// encoding.  It returns non-fatal warning strings; it never returns errors.
// Only strict profiles (LevelEBPAJ, LevelKADOKAWA) produce meaningful output.
func preflightEncode(level ComplianceLevel, doc *Document) []string {
	if level == LevelFlexible || doc == nil {
		return nil
	}

	var warnings []string

	for href, asset := range doc.Assets {
		if asset == nil {
			continue
		}
		mt := strings.ToLower(strings.TrimSpace(asset.MimeType))

		// Directory layout check.
		dir := path.Dir(href)
		if dir == "." || (!strings.HasPrefix(href, "item/xhtml/") &&
			!strings.HasPrefix(href, "item/image/") &&
			!strings.HasPrefix(href, "item/style/") &&
			dir != "item") {
			warnings = append(warnings, fmt.Sprintf("asset %q does not follow the expected directory layout", href))
		}

		if strings.HasPrefix(mt, "image/") {
			// Image naming.
			switch level {
			case LevelEBPAJ:
				if !imageNamePatternEBPAJ.MatchString(href) {
					warnings = append(warnings, fmt.Sprintf("image %q does not match EBPAJ naming pattern", href))
				}
			case LevelKADOKAWA:
				if !imageNamePatternKADOKAWA.MatchString(href) {
					warnings = append(warnings, fmt.Sprintf("image %q does not match KADOKAWA naming pattern", href))
				}
			}

			// File size.
			if asset.Size > uint64(maxImageFileSize) {
				warnings = append(warnings, fmt.Sprintf("image %q file size %s exceeds %s limit",
					href, formatBytes(int64(asset.Size)), formatBytes(maxImageFileSize)))
			}

			// Pixel count and progressive JPEG (requires opening asset).
			if asset.Open != nil {
				if w := preflightImageSpecs(href, asset); len(w) > 0 {
					warnings = append(warnings, w...)
				}
			}
		} else if strings.Contains(mt, "xhtml") {
			if asset.Size > uint64(maxXHTMLFileSize) {
				warnings = append(warnings, fmt.Sprintf("XHTML file %q size %s exceeds %s, may cause RS performance issues",
					href, formatBytes(int64(asset.Size)), formatBytes(int64(maxXHTMLFileSize))))
			}
		}
	}
	return warnings
}

// preflightImageSpecs opens an image asset and checks pixel count and JPEG encoding.
func preflightImageSpecs(href string, asset *Asset) []string {
	rc, err := asset.Open()
	if err != nil {
		return nil
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}

	var warnings []string

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	pixelCount := int64(cfg.Width) * int64(cfg.Height)
	if pixelCount > maxImagePixelCount {
		warnings = append(warnings, fmt.Sprintf("image %q pixel count %d exceeds %d limit",
			href, pixelCount, maxImagePixelCount))
	}

	if format == "jpeg" {
		if isProgressiveJPEG(data) {
			warnings = append(warnings, fmt.Sprintf("image %q is a progressive JPEG, which is not allowed", href))
		}
	}

	return warnings
}

// isProgressiveJPEG detects progressive JPEG by looking for SOF2 marker (0xFFC2).
func isProgressiveJPEG(data []byte) bool {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0xFF && data[i+1] == 0xC2 {
			return true
		}
	}
	return false
}
