// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidMimeType is returned when the mimetype file is missing or has an invalid value.
	ErrInvalidMimeType = errors.New("mimetype must be 'application/epub+zip' and uncompressed")
	// ErrMimeTypeNotFirst is returned when mimetype is not the first ZIP entry.
	ErrMimeTypeNotFirst = errors.New("mimetype must be the first ZIP entry")
	// ErrContainerNotFound is returned when the container.xml file is missing.
	ErrContainerNotFound = errors.New("META-INF/container.xml not found")
	// ErrOPFNotFound is returned when OPF rootfile is missing.
	ErrOPFNotFound = errors.New("OPF rootfile not found")
	// ErrManifestMissing is returned when OPF manifest is not present.
	ErrManifestMissing = errors.New("OPF manifest is missing")
	// ErrSpineMissing is returned when OPF spine is not present.
	ErrSpineMissing = errors.New("OPF spine is missing")
	// ErrInvalidViewport is returned when viewport meta cannot be parsed.
	ErrInvalidViewport = errors.New("invalid viewport meta")
	// ErrAssetNotFound is returned when an asset cannot be resolved from the current document.
	ErrAssetNotFound = errors.New("asset not found")
	// ErrNilAsset is returned when asset receiver is nil.
	ErrNilAsset = errors.New("asset is nil")
	// ErrNilAssetOpen is returned when asset stream function is nil.
	ErrNilAssetOpen = errors.New("asset Open function is nil")
	// ErrNilDocument is returned when document receiver is nil.
	ErrNilDocument = errors.New("document is nil")
	// ErrNilPage is returned when page argument is nil.
	ErrNilPage = errors.New("page is nil")
	// ErrEmptyAssetID is returned when page asset id is empty.
	ErrEmptyAssetID = errors.New("asset id is empty")
	// ErrInvalidSpread is returned when page spread is unsupported.
	ErrInvalidSpread = errors.New("spread must be one of left, right, center, none")
	// ErrEmptyAssetPath is returned when asset href/path is empty.
	ErrEmptyAssetPath = errors.New("asset href is empty")
	// ErrDuplicateAssetPath is returned when the same href/path is already registered.
	ErrDuplicateAssetPath = errors.New("asset href is already registered")
	// ErrDuplicateAssetID is returned when the same asset id is already registered.
	ErrDuplicateAssetID = errors.New("asset id is already registered")
	// ErrEmptyMimeType is returned when asset media type is empty.
	ErrEmptyMimeType = errors.New("asset mime type is empty")
	// ErrNilReaderAt is returned when io.ReaderAt source is nil.
	ErrNilReaderAt = errors.New("asset readerAt is nil")
	// ErrInvalidAssetSize is returned when asset size is negative.
	ErrInvalidAssetSize = errors.New("asset size must be >= 0")
	// ErrCannotInferAssetPath is returned when href is empty and mime type cannot be mapped to an extension.
	ErrCannotInferAssetPath = errors.New("cannot infer asset href from mime type")
	// ErrCannotInferAssetSize is returned when a ReaderAt length cannot be inferred.
	ErrCannotInferAssetSize = errors.New("cannot infer asset size from readerAt")

	errInvalidImageNaming     = errors.New("image path must follow item/image/p-000.(jpg|png) format")
	errInvalidDirectoryLayout = errors.New("asset path must be under item/xhtml, item/image, or item/style")
)

// ErrSpineUnknownIDRef is returned when a spine itemref points to an unknown manifest ID.
type ErrSpineUnknownIDRef struct {
	IDRef string
}

func (e *ErrSpineUnknownIDRef) Error() string {
	if e == nil || e.IDRef == "" {
		return "spine itemref references unknown manifest id"
	}
	return fmt.Sprintf("spine itemref references unknown manifest id %q", e.IDRef)
}

func (e *ErrSpineUnknownIDRef) Is(target error) bool {
	_, ok := target.(*ErrSpineUnknownIDRef)
	return ok
}

// ErrManifestPhysicalMissing is returned when a manifest item href does not exist in the ZIP.
type ErrManifestPhysicalMissing struct {
	Href string
}

func (e *ErrManifestPhysicalMissing) Error() string {
	if e == nil || e.Href == "" {
		return "manifest item href does not exist in ZIP"
	}
	return fmt.Sprintf("manifest item href %q does not exist in ZIP", e.Href)
}

func (e *ErrManifestPhysicalMissing) Is(target error) bool {
	_, ok := target.(*ErrManifestPhysicalMissing)
	return ok
}

// ErrMaxAssetCountExceeded is returned when ZIP entry count exceeds configured limit.
type ErrMaxAssetCountExceeded struct {
	Limit  int
	Actual int
}

func (e *ErrMaxAssetCountExceeded) Error() string {
	if e == nil {
		return "zip entry count exceeds configured limit"
	}
	return fmt.Sprintf("zip entry count exceeds configured limit: actual=%d limit=%d", e.Actual, e.Limit)
}

func (e *ErrMaxAssetCountExceeded) Is(target error) bool {
	_, ok := target.(*ErrMaxAssetCountExceeded)
	return ok
}

// ErrMaxTotalUncompressedSizeExceeded is returned when total ZIP uncompressed size exceeds configured limit.
type ErrMaxTotalUncompressedSizeExceeded struct {
	Limit  int64
	Actual uint64
}

func (e *ErrMaxTotalUncompressedSizeExceeded) Error() string {
	if e == nil {
		return "total uncompressed size exceeds configured limit"
	}
	return fmt.Sprintf("total uncompressed size exceeds configured limit: actual=%d limit=%d", e.Actual, e.Limit)
}

func (e *ErrMaxTotalUncompressedSizeExceeded) Is(target error) bool {
	_, ok := target.(*ErrMaxTotalUncompressedSizeExceeded)
	return ok
}

// ErrMaxIndividualAssetSizeExceeded is returned when a ZIP entry uncompressed size exceeds configured limit.
type ErrMaxIndividualAssetSizeExceeded struct {
	Name   string
	Limit  int64
	Actual uint64
}

func (e *ErrMaxIndividualAssetSizeExceeded) Error() string {
	if e == nil {
		return "individual uncompressed size exceeds configured limit"
	}
	if e.Name == "" {
		return fmt.Sprintf("individual uncompressed size exceeds configured limit: actual=%d limit=%d", e.Actual, e.Limit)
	}
	return fmt.Sprintf("individual uncompressed size exceeds configured limit for %q: actual=%d limit=%d", e.Name, e.Actual, e.Limit)
}

func (e *ErrMaxIndividualAssetSizeExceeded) Is(target error) bool {
	_, ok := target.(*ErrMaxIndividualAssetSizeExceeded)
	return ok
}

// DecodeError represents an error that occurred during the decoding of an EPUB file, including the path where the error occurred and the underlying error.
type DecodeError struct {
	Path string
	Rule string
	Err  error
}

func (e *DecodeError) Error() string {
	if e.Rule != "" {
		return fmt.Sprintf("epub decode error at [%s] (%s): %v", e.Path, e.Rule, e.Err)
	}
	return fmt.Sprintf("epub decode error at [%s]: %v", e.Path, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }

// ErrImagePixelCountExceeded is returned when image pixel count exceeds the limit.
type ErrImagePixelCountExceeded struct {
	Href   string
	Limit  int64
	Actual int64
}

func (e *ErrImagePixelCountExceeded) Error() string {
	if e == nil || e.Href == "" {
		return "image pixel count exceeds limit"
	}
	return fmt.Sprintf("image %q pixel count exceeds limit: %d > %d", e.Href, e.Actual, e.Limit)
}

func (e *ErrImagePixelCountExceeded) Is(target error) bool {
	_, ok := target.(*ErrImagePixelCountExceeded)
	return ok
}

// ErrImageFileSizeTooLarge is returned when image file size exceeds the limit.
type ErrImageFileSizeTooLarge struct {
	Href   string
	Limit  int64
	Actual int64
}

func (e *ErrImageFileSizeTooLarge) Error() string {
	if e == nil || e.Href == "" {
		return "image file size exceeds limit"
	}
	return fmt.Sprintf("image %q file size exceeds limit: %d > %d bytes", e.Href, e.Actual, e.Limit)
}

func (e *ErrImageFileSizeTooLarge) Is(target error) bool {
	_, ok := target.(*ErrImageFileSizeTooLarge)
	return ok
}

// ErrProgressiveJPEGNotAllowed is returned when a progressive JPEG is detected.
type ErrProgressiveJPEGNotAllowed struct {
	Href string
}

func (e *ErrProgressiveJPEGNotAllowed) Error() string {
	if e == nil || e.Href == "" {
		return "progressive JPEG is not allowed"
	}
	return fmt.Sprintf("image %q: progressive JPEG is not allowed", e.Href)
}

func (e *ErrProgressiveJPEGNotAllowed) Is(target error) bool {
	_, ok := target.(*ErrProgressiveJPEGNotAllowed)
	return ok
}

// ErrInvalidColorSpace is returned when image has invalid color space.
type ErrInvalidColorSpace struct {
	Href     string
	Expected string
	Actual   string
}

func (e *ErrInvalidColorSpace) Error() string {
	if e == nil || e.Href == "" {
		return "invalid color space"
	}
	return fmt.Sprintf("image %q has invalid color space: expected %s, got %s", e.Href, e.Expected, e.Actual)
}

func (e *ErrInvalidColorSpace) Is(target error) bool {
	_, ok := target.(*ErrInvalidColorSpace)
	return ok
}
