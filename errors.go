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
