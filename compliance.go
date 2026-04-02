// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"path"
	"regexp"
	"strings"
)

var imageNamePatternEBPAJ = regexp.MustCompile(`^item/image/p-[0-9]{3,4}\.(jpg|png)$`)

// KADOKAWA spec §2: ASCII lowercase alphanumeric, hyphen, underscore; starts with a letter.
var imageNamePatternKADOKAWA = regexp.MustCompile(`^item/image/[a-z][a-z0-9_-]*\.(jpg|jpeg|png|gif)$`)

func validateCompliance(level ComplianceLevel, files map[string]struct{}, manifest map[string]manifestItem) error {
	if level == LevelFlexible {
		return nil
	}

	for href, item := range manifest {
		if _, ok := files[href]; !ok {
			return &DecodeError{Path: href, Rule: "manifest-physical-existence", Err: &ErrManifestPhysicalMissing{Href: href}}
		}
		if err := validateDirectoryRule(href); err != nil {
			return err
		}
		if strings.HasPrefix(item.MediaType, "image/") {
			if err := validateImageNaming(level, href); err != nil {
				return err
			}
		}
	}
	return nil
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
