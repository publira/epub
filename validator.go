// SPDX-License-Identifier: Apache-2.0

package epub

// Validator validates EPUB document and assets.
//
// Implementations reside in profile sub-packages such as
// [github.com/publira/epub/profile/ebpaj],
// [github.com/publira/epub/profile/kadokawa], and
// [github.com/publira/epub/profile/kindle].
type Validator interface {
	ValidateDocument(doc *Document) []error
	ValidateAsset(href string, asset *Asset) []error
}

// ValidationWarningError represents a non-fatal validation issue.
// Validators may return *ValidationWarningError to signal the issue should be
// treated as a warning rather than a fatal error during Decode.
// During Encode preflight all errors (including non-warning) are treated
// as warnings.
type ValidationWarningError struct {
	Message string
}

func (w *ValidationWarningError) Error() string { return w.Message }
