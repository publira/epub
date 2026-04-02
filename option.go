// SPDX-License-Identifier: Apache-2.0

package epub

// ComplianceLevel controls strictness of EPUB validations.
type ComplianceLevel int

const (
	// LevelFlexible allows broader structures as long as package integrity is valid.
	LevelFlexible ComplianceLevel = iota
	// LevelEBPAJ enforces EBPAJ-oriented naming and directory conventions.
	LevelEBPAJ
	// LevelKADOKAWA enforces KADOKAWA-oriented naming and directory conventions.
	LevelKADOKAWA
)

// decodeConfig stores Decode options.
type decodeConfig struct {
	compliance               ComplianceLevel
	maxAssetCount            int
	maxTotalUncompressedSize int64
	maxIndividualAssetSize   int64
}

// DecodeOption mutates decode behavior.
type DecodeOption func(*decodeConfig)

// WithCompliance configures strictness level used by Decode.
func WithCompliance(level ComplianceLevel) DecodeOption {
	return func(cfg *decodeConfig) {
		cfg.compliance = level
	}
}

// WithMaxAssetCount limits the number of ZIP entries accepted during Decode.
// A value <= 0 disables this limit.
func WithMaxAssetCount(max int) DecodeOption {
	return func(cfg *decodeConfig) {
		cfg.maxAssetCount = max
	}
}

// WithMaxTotalUncompressedSize limits the sum of all ZIP entry uncompressed sizes accepted during Decode.
// A value <= 0 disables this limit.
func WithMaxTotalUncompressedSize(max int64) DecodeOption {
	return func(cfg *decodeConfig) {
		cfg.maxTotalUncompressedSize = max
	}
}

// WithMaxIndividualAssetSize limits each ZIP entry uncompressed size accepted during Decode.
// A value <= 0 disables this limit.
func WithMaxIndividualAssetSize(max int64) DecodeOption {
	return func(cfg *decodeConfig) {
		cfg.maxIndividualAssetSize = max
	}
}

func defaultDecodeConfig() decodeConfig {
	return decodeConfig{compliance: LevelFlexible}
}
