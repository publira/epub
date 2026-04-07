// SPDX-License-Identifier: Apache-2.0

package epub

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const mimeTypeValue = "application/epub+zip"

var viewportPattern = regexp.MustCompile(`(?i)width\s*=\s*([0-9]+)\s*,\s*height\s*=\s*([0-9]+)`)

// Decode parses EPUB from ReaderAt without relying on filesystem paths.
func Decode(r io.ReaderAt, size int64, opts ...DecodeOption) (*Document, error) {
	cfg := defaultDecodeConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, &DecodeError{Path: "zip", Rule: "zip-open", Err: err}
	}
	if err := validateResourceLimits(zr, cfg); err != nil {
		return nil, err
	}

	if err := validateMimeType(zr); err != nil {
		return nil, err
	}

	filesByName := make(map[string]*zip.File, len(zr.File))
	fileSet := make(map[string]struct{}, len(zr.File))
	for _, f := range zr.File {
		name := path.Clean(strings.TrimPrefix(f.Name, "./"))
		filesByName[name] = f
		fileSet[name] = struct{}{}
	}

	opfPath, err := readOPFPath(filesByName)
	if err != nil {
		return nil, err
	}

	pkg, err := readPackageXML(filesByName, opfPath)
	if err != nil {
		return nil, err
	}
	if len(pkg.Manifest.Items) == 0 {
		return nil, &DecodeError{Path: opfPath, Rule: "manifest", Err: ErrManifestMissing}
	}
	if len(pkg.Spine.Itemrefs) == 0 {
		return nil, &DecodeError{Path: opfPath, Rule: "spine", Err: ErrSpineMissing}
	}

	manifestByID := make(map[string]manifestItem, len(pkg.Manifest.Items))
	manifestByHref := make(map[string]manifestItem, len(pkg.Manifest.Items))
	normalizedManifest := make([]manifestItem, 0, len(pkg.Manifest.Items))
	warnings := make([]string, 0)
	warningSet := make(map[string]struct{})
	addWarning := func(msg string) {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			return
		}
		if _, exists := warningSet[msg]; exists {
			return
		}
		warningSet[msg] = struct{}{}
		warnings = append(warnings, msg)
	}
	opfDir := path.Dir(opfPath)
	for i, it := range pkg.Manifest.Items {
		href := cleanOPFRef(opfDir, it.Href)
		it.Href = href
		pkg.Manifest.Items[i] = it
		if prev, exists := manifestByID[it.ID]; exists {
			addWarning(fmt.Sprintf("duplicate manifest id %q: href %q ignored (already defined as %q)", it.ID, it.Href, prev.Href))
			continue
		}
		if prev, exists := manifestByHref[href]; exists {
			addWarning(fmt.Sprintf("duplicate manifest href %q: id %q ignored (already defined as %q)", it.Href, it.ID, prev.ID))
			continue
		}
		manifestByID[it.ID] = it
		manifestByHref[href] = it
		normalizedManifest = append(normalizedManifest, it)
	}

	complianceWarnings, err := validateCompliance(cfg.compliance, fileSet, manifestByHref, filesByName)
	if err != nil {
		return nil, err
	}

	title, titleID := primaryDCValue(pkg.Metadata.Titles)
	identifier, identifierID := primaryIdentifier(pkg.Metadata.Identifiers, pkg.UniqueIdentifier)
	language, _ := primaryDCValue(pkg.Metadata.Languages)
	fileAsByRefines := parseFileAsByRefines(pkg.Metadata.Meta)

	titleFileAs := ""
	if titleID != "" {
		titleFileAs = fileAsByRefines["#"+titleID]
	}

	doc := &Document{
		Metadata: Metadata{
			Title:                title,
			TitleFileAs:          titleFileAs,
			Identifier:           identifier,
			IdentifierID:         identifierID,
			Language:             language,
			Creators:             parseCreators(pkg.Metadata.Creators, fileAsByRefines),
			CoverAssetID:         parseCoverAssetID(normalizedManifest, pkg.Metadata.Meta),
			RenditionSpread:      parseMetaValueByProperty(pkg.Metadata.Meta, "rendition:spread"),
			RenditionOrientation: parseMetaValueByProperty(pkg.Metadata.Meta, "rendition:orientation"),
			EBPAJGuideVersion:    parseMetaValueByProperty(pkg.Metadata.Meta, "ebpaj:guide-version"),
			KADOKAWAVersion:      parseMetaValueByProperty(pkg.Metadata.Meta, "kadokawa:version"),
		},
		Title:      title,
		Identifier: identifier,
		Direction:  normalizeDirection(pkg.Spine.PageProgressionDirection),
		Layout:     parseLayoutType(pkg.Metadata.Meta),
		Pages:      make([]*Page, 0, len(pkg.Spine.Itemrefs)),
		Assets:     make(map[string]*Asset, len(normalizedManifest)),
		Warnings:   append(warnings, *complianceWarnings...),
	}

	for _, item := range normalizedManifest {
		zfile, ok := filesByName[item.Href]
		if !ok {
			return nil, &DecodeError{Path: item.Href, Rule: "manifest-physical-existence", Err: &ManifestPhysicalMissingError{Href: item.Href}}
		}
		current := zfile
		asset := &Asset{
			ID:       item.ID,
			MimeType: item.MediaType,
			Size:     current.UncompressedSize64,
			Open: func() (io.ReadCloser, error) {
				return current.Open()
			},
		}
		if err := asset.CalculateHash(); err != nil {
			return nil, &DecodeError{Path: item.Href, Rule: "asset-checksum", Err: err}
		}
		doc.Assets[item.Href] = asset
	}

	for i, ref := range pkg.Spine.Itemrefs {
		item, ok := manifestByID[ref.IDRef]
		if !ok {
			return nil, &DecodeError{Path: opfPath, Rule: "spine-idref", Err: &SpineUnknownIDRefError{IDRef: ref.IDRef}}
		}
		page := &Page{Order: i, AssetID: item.ID, Href: item.Href, Spread: spreadFromProperties(ref.Properties)}
		if doc.IsPrePaginated() && strings.Contains(item.MediaType, "xhtml") {
			width, height, err := readViewport(filesByName, item.Href)
			if err != nil {
				return nil, err
			}
			page.Width = width
			page.Height = height
		}
		doc.Pages = append(doc.Pages, page)
	}

	// Mark cover page based on CoverAssetID.
	if coverID := doc.Metadata.CoverAssetID; coverID != "" {
		wrapperID := "xhtml-" + coverID
		for _, pg := range doc.Pages {
			if pg.AssetID == wrapperID || pg.AssetID == coverID {
				pg.Type = PageTypeCover
				break
			}
		}
	}

	reservedPaths := map[string]struct{}{
		"mimetype":               {},
		"META-INF/container.xml": {},
		opfPath:                  {},
	}
	zipNames := make([]string, 0, len(fileSet))
	for name := range fileSet {
		zipNames = append(zipNames, name)
	}
	sort.Strings(zipNames)
	for _, name := range zipNames {
		if _, isReserved := reservedPaths[name]; isReserved {
			continue
		}
		if _, exists := manifestByHref[name]; !exists {
			addWarning(fmt.Sprintf("archive file %q is not declared in manifest", name))
		}
	}

	doc.Warnings = warnings

	return doc, nil
}

func validateMimeType(zr *zip.Reader) error {
	if len(zr.File) == 0 || zr.File[0].Name != "mimetype" {
		return &DecodeError{Path: "mimetype", Rule: "mimetype-first", Err: ErrMimeTypeNotFirst}
	}
	f := zr.File[0]
	if f.Method != zip.Store {
		return &DecodeError{Path: "mimetype", Rule: "mimetype-compression", Err: ErrInvalidMimeType}
	}
	rc, err := f.Open()
	if err != nil {
		return &DecodeError{Path: "mimetype", Rule: "mimetype-read", Err: err}
	}
	defer func() { _ = rc.Close() }()
	buf, err := io.ReadAll(rc)
	if err != nil {
		return &DecodeError{Path: "mimetype", Rule: "mimetype-read", Err: err}
	}
	if strings.TrimSpace(string(buf)) != mimeTypeValue {
		return &DecodeError{Path: "mimetype", Rule: "mimetype-content", Err: ErrInvalidMimeType}
	}
	return nil
}

func readOPFPath(files map[string]*zip.File) (string, error) {
	container, ok := files["META-INF/container.xml"]
	if !ok {
		return "", &DecodeError{Path: "META-INF/container.xml", Rule: "container-file", Err: ErrContainerNotFound}
	}
	rc, err := container.Open()
	if err != nil {
		return "", &DecodeError{Path: "META-INF/container.xml", Rule: "container-read", Err: err}
	}
	defer func() { _ = rc.Close() }()

	var c containerXML
	if err := xml.NewDecoder(rc).Decode(&c); err != nil {
		return "", &DecodeError{Path: "META-INF/container.xml", Rule: "container-xml", Err: err}
	}
	if len(c.Rootfiles.Rootfile) == 0 {
		return "", &DecodeError{Path: "META-INF/container.xml", Rule: "container-rootfile", Err: ErrOPFNotFound}
	}
	opfPath := path.Clean(c.Rootfiles.Rootfile[0].FullPath)
	if opfPath == "." || opfPath == "" {
		return "", &DecodeError{Path: "META-INF/container.xml", Rule: "container-rootfile", Err: ErrOPFNotFound}
	}
	return opfPath, nil
}

func readPackageXML(files map[string]*zip.File, opfPath string) (*packageXML, error) {
	f, ok := files[opfPath]
	if !ok {
		return nil, &DecodeError{Path: opfPath, Rule: "opf-file", Err: ErrOPFNotFound}
	}
	rc, err := f.Open()
	if err != nil {
		return nil, &DecodeError{Path: opfPath, Rule: "opf-read", Err: err}
	}
	defer func() { _ = rc.Close() }()

	var pkg packageXML
	if err := xml.NewDecoder(rc).Decode(&pkg); err != nil {
		return nil, &DecodeError{Path: opfPath, Rule: "opf-xml", Err: err}
	}
	return &pkg, nil
}

func readViewport(files map[string]*zip.File, href string) (int, int, error) {
	f, ok := files[href]
	if !ok {
		return 0, 0, &DecodeError{Path: href, Rule: "viewport-file", Err: &ManifestPhysicalMissingError{Href: href}}
	}
	rc, err := f.Open()
	if err != nil {
		return 0, 0, &DecodeError{Path: href, Rule: "viewport-read", Err: err}
	}
	defer func() { _ = rc.Close() }()
	buf, err := io.ReadAll(rc)
	if err != nil {
		return 0, 0, &DecodeError{Path: href, Rule: "viewport-read", Err: err}
	}

	content, err := extractViewportMeta(string(buf))
	if err != nil {
		return 0, 0, &DecodeError{Path: href, Rule: "viewport-meta", Err: err}
	}
	m := viewportPattern.FindStringSubmatch(content)
	if len(m) != 3 {
		return 0, 0, &DecodeError{Path: href, Rule: "viewport-format", Err: ErrInvalidViewport}
	}
	w, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, &DecodeError{Path: href, Rule: "viewport-width", Err: err}
	}
	h, err := strconv.Atoi(m[2])
	if err != nil {
		return 0, 0, &DecodeError{Path: href, Rule: "viewport-height", Err: err}
	}
	return w, h, nil
}

func extractViewportMeta(xhtml string) (string, error) {
	d := xml.NewDecoder(strings.NewReader(xhtml))
	for {
		tok, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || !strings.EqualFold(se.Name.Local, "meta") {
			continue
		}
		name := ""
		content := ""
		for _, a := range se.Attr {
			switch strings.ToLower(a.Name.Local) {
			case "name":
				name = a.Value
			case "content":
				content = a.Value
			}
		}
		if strings.EqualFold(name, "viewport") {
			return content, nil
		}
	}
	return "", ErrInvalidViewport
}

func spreadFromProperties(properties string) string {
	switch {
	case strings.Contains(properties, "page-spread-left"):
		return "left"
	case strings.Contains(properties, "page-spread-right"):
		return "right"
	case strings.Contains(properties, "page-spread-center"):
		return "center"
	default:
		return "none"
	}
}

func parseLayoutType(meta []metadataMeta) LayoutType {
	for _, m := range meta {
		if strings.EqualFold(strings.TrimSpace(m.Property), "rendition:layout") {
			v := strings.TrimSpace(m.Value)
			if v == "" {
				v = strings.TrimSpace(m.Content)
			}
			switch {
			case strings.EqualFold(v, "pre-paginated"):
				return LayoutPrePaginated
			case strings.EqualFold(v, "reflowable"):
				return LayoutReflowable
			default:
				return LayoutUnknown
			}
		}
	}
	return LayoutUnknown
}

func parseMetaValueByProperty(meta []metadataMeta, property string) string {
	for _, m := range meta {
		if strings.EqualFold(strings.TrimSpace(m.Property), property) {
			v := strings.TrimSpace(m.Value)
			if v == "" {
				v = strings.TrimSpace(m.Content)
			}
			return v
		}
	}
	return ""
}

func parseFileAsByRefines(meta []metadataMeta) map[string]string {
	result := make(map[string]string)
	for _, m := range meta {
		if !strings.EqualFold(strings.TrimSpace(m.Property), "file-as") {
			continue
		}
		refines := strings.TrimSpace(m.Refines)
		if refines == "" {
			continue
		}
		v := strings.TrimSpace(m.Value)
		if v == "" {
			v = strings.TrimSpace(m.Content)
		}
		if v == "" {
			continue
		}
		result[refines] = v
	}
	return result
}

func parseCreators(creators []dcElement, fileAsByRefines map[string]string) []Creator {
	result := make([]Creator, 0, len(creators))
	for _, creator := range creators {
		name := strings.TrimSpace(creator.Value)
		if name == "" {
			continue
		}
		ref := ""
		if id := strings.TrimSpace(creator.ID); id != "" {
			ref = "#" + id
		}
		result = append(result, Creator{Name: name, FileAs: fileAsByRefines[ref]})
	}
	return result
}

func primaryDCValue(values []dcElement) (string, string) {
	for _, v := range values {
		value := strings.TrimSpace(v.Value)
		if value != "" {
			return value, strings.TrimSpace(v.ID)
		}
	}
	return "", ""
}

func primaryIdentifier(values []dcElement, preferredID string) (string, string) {
	preferredID = strings.TrimSpace(preferredID)
	if preferredID != "" {
		for _, v := range values {
			id := strings.TrimSpace(v.ID)
			if id != preferredID {
				continue
			}
			value := strings.TrimSpace(v.Value)
			if value != "" {
				return value, id
			}
		}
	}

	for _, v := range values {
		value := strings.TrimSpace(v.Value)
		if value != "" {
			return value, strings.TrimSpace(v.ID)
		}
	}
	return "", preferredID
}

func normalizeDirection(v string) string {
	if strings.EqualFold(v, "rtl") {
		return "rtl"
	}
	return "ltr"
}

func cleanOPFRef(opfDir, href string) string {
	if strings.HasPrefix(href, "/") {
		return path.Clean(strings.TrimPrefix(href, "/"))
	}
	joined := path.Clean(path.Join(opfDir, href))
	return strings.TrimPrefix(joined, "./")
}

func validateResourceLimits(zr *zip.Reader, cfg decodeConfig) error {
	if cfg.maxAssetCount > 0 && len(zr.File) > cfg.maxAssetCount {
		return &DecodeError{
			Path: "zip",
			Rule: "max-asset-count",
			Err:  &MaxAssetCountExceededError{Limit: cfg.maxAssetCount, Actual: len(zr.File)},
		}
	}

	var total uint64
	for _, f := range zr.File {
		size := f.UncompressedSize64
		if cfg.maxIndividualAssetSize > 0 && size > uint64(cfg.maxIndividualAssetSize) {
			return &DecodeError{
				Path: f.Name,
				Rule: "max-individual-asset-size",
				Err:  &MaxIndividualAssetSizeExceededError{Name: f.Name, Limit: cfg.maxIndividualAssetSize, Actual: size},
			}
		}

		total += size
		if cfg.maxTotalUncompressedSize > 0 && total > uint64(cfg.maxTotalUncompressedSize) {
			return &DecodeError{
				Path: "zip",
				Rule: "max-total-uncompressed-size",
				Err:  &MaxTotalUncompressedSizeExceededError{Limit: cfg.maxTotalUncompressedSize, Actual: total},
			}
		}
	}

	return nil
}

func parseCoverAssetID(manifest []manifestItem, meta []metadataMeta) string {
	for _, item := range manifest {
		for _, prop := range strings.Fields(item.Properties) {
			if strings.EqualFold(prop, "cover-image") {
				return item.ID
			}
		}
	}
	for _, m := range meta {
		if strings.EqualFold(strings.TrimSpace(m.Name), "cover") {
			if v := strings.TrimSpace(m.Content); v != "" {
				return v
			}
		}
	}
	return ""
}
