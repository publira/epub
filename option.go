// SPDX-License-Identifier: Apache-2.0

package epub

// ComplianceLevel controls strictness of EPUB validations.
//
// Deprecated: Use [WithValidator] with profile sub-packages instead.
type ComplianceLevel int

const (
	// LevelFlexible allows broader structures as long as package integrity is valid.
	//
	// Deprecated: Use [WithValidator] with profile sub-packages instead.
	LevelFlexible ComplianceLevel = iota
	// LevelEBPAJ enforces EBPAJ-oriented naming and directory conventions.
	//
	// Deprecated: Use [WithValidator] with [github.com/publira/epub/profile/ebpaj] instead.
	LevelEBPAJ
	// LevelKADOKAWA enforces KADOKAWA-oriented naming and directory conventions.
	//
	// Deprecated: Use [WithValidator] with [github.com/publira/epub/profile/kadokawa] instead.
	LevelKADOKAWA
	// LevelKindle enforces Amazon Kindle (KDP) publishing guidelines.
	//
	// Deprecated: Use [WithValidator] with [github.com/publira/epub/profile/kindle] instead.
	LevelKindle
)

// decodeConfig stores Decode options.
type decodeConfig struct {
	compliance               ComplianceLevel
	validators               []Validator
	maxAssetCount            int
	maxTotalUncompressedSize int64
	maxIndividualAssetSize   int64
}

// DecodeOption configures Decode behavior.
type DecodeOption interface {
	applyDecode(*decodeConfig)
}

// decodeOptionFunc wraps a function as a [DecodeOption].
type decodeOptionFunc func(*decodeConfig)

func (f decodeOptionFunc) applyDecode(c *decodeConfig) { f(c) }

// EncodeOption configures Encode behavior.
type EncodeOption interface {
	applyEncode(*encodeConfig)
}

// encodeOptionFunc wraps a function as an [EncodeOption].
type encodeOptionFunc func(*encodeConfig)

func (f encodeOptionFunc) applyEncode(c *encodeConfig) { f(c) }

// Option applies to both Decode and Encode.
type Option interface {
	DecodeOption
	EncodeOption
}

// optionFunc implements [Option].
type optionFunc struct {
	decode func(*decodeConfig)
	encode func(*encodeConfig)
}

func (f optionFunc) applyDecode(c *decodeConfig) {
	if f.decode != nil {
		f.decode(c)
	}
}

func (f optionFunc) applyEncode(c *encodeConfig) {
	if f.encode != nil {
		f.encode(c)
	}
}

// WithCompliance configures strictness level used by Decode.
//
// Deprecated: Use [WithValidator] with profile sub-packages instead.
func WithCompliance(level ComplianceLevel) DecodeOption {
	return decodeOptionFunc(func(cfg *decodeConfig) {
		cfg.compliance = level
	})
}

// WithValidator registers one or more [Validator] implementations.
//
// When passed to [Decode], validators are run after the [Document] is fully
// constructed. Errors implementing [*ValidationWarningError] are collected
// into [Document.Warnings]; all other errors cause Decode to fail.
//
// When passed to [Encode], validators are executed as preflight checks before
// writing the EPUB ZIP. All errors (including non-warning) are delivered as
// warnings through the collector registered via [WithEncodeWarningCollector].
//
// Example:
//
//	doc, err := epub.Decode(f, size,
//	    epub.WithValidator(kadokawa.New(), kindle.New()),
//	)
func WithValidator(validators ...Validator) Option {
	return optionFunc{
		decode: func(c *decodeConfig) {
			c.validators = append(c.validators, validators...)
		},
		encode: func(c *encodeConfig) {
			c.preflightValidators = append(c.preflightValidators, validators...)
		},
	}
}

// WithMaxAssetCount limits the number of ZIP entries accepted during Decode.
// A value <= 0 disables this limit.
func WithMaxAssetCount(max int) DecodeOption {
	return decodeOptionFunc(func(cfg *decodeConfig) {
		cfg.maxAssetCount = max
	})
}

// WithMaxTotalUncompressedSize limits the sum of all ZIP entry uncompressed sizes accepted during Decode.
// A value <= 0 disables this limit.
func WithMaxTotalUncompressedSize(max int64) DecodeOption {
	return decodeOptionFunc(func(cfg *decodeConfig) {
		cfg.maxTotalUncompressedSize = max
	})
}

// WithMaxIndividualAssetSize limits each ZIP entry uncompressed size accepted during Decode.
// A value <= 0 disables this limit.
func WithMaxIndividualAssetSize(max int64) DecodeOption {
	return decodeOptionFunc(func(cfg *decodeConfig) {
		cfg.maxIndividualAssetSize = max
	})
}

func defaultDecodeConfig() decodeConfig {
	return decodeConfig{compliance: LevelFlexible}
}
