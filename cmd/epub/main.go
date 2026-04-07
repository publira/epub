// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/publira/epub"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "inspect":
		if err := runInspect(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "inspect failed: %v\n", err)
			os.Exit(1)
		}
	case "repack":
		if err := runRepack(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "repack failed: %v\n", err)
			os.Exit(1)
		}
	case "images":
		if err := runListImages(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "images failed: %v\n", err)
			os.Exit(1)
		}
	case "build-images":
		if err := runBuildFromImages(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "build-images failed: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	inPath := fs.String("in", "", "input .epub path")
	compliance := fs.String("compliance", "flexible", "flexible|ebpaj|kadokawa")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*inPath) == "" {
		return fmt.Errorf("-in is required")
	}

	f, size, err := openReaderAt(*inPath)
	if err != nil {
		return err
	}
	defer f.Close()

	level, err := parseCompliance(*compliance)
	if err != nil {
		return err
	}
	doc, err := epub.Decode(f, size, epub.WithCompliance(level))
	if err != nil {
		return err
	}

	fmt.Printf("Title: %s\n", doc.Metadata.Title)
	fmt.Printf("Direction: %s\n", doc.Direction)
	fmt.Printf("Layout: %s\n", doc.Layout.String())
	fmt.Printf("Pages: %d\n", len(doc.Pages))
	fmt.Printf("Assets: %d\n", len(doc.Assets))

	if len(doc.Pages) > 0 {
		fmt.Println("-- First pages --")
		limit := 3
		if len(doc.Pages) < limit {
			limit = len(doc.Pages)
		}
		for i := 0; i < limit; i++ {
			p := doc.Pages[i]
			fmt.Printf("[%d] asset=%s href=%s spread=%s size=%dx%d\n", p.Order, p.AssetID, p.Href, p.Spread, p.Width, p.Height)
		}
	}

	if len(doc.Assets) > 0 {
		fmt.Println("-- Asset keys (first 5) --")
		keys := make([]string, 0, len(doc.Assets))
		for k := range doc.Assets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		limit := 5
		if len(keys) < limit {
			limit = len(keys)
		}
		for i := 0; i < limit; i++ {
			a := doc.Assets[keys[i]]
			fmt.Printf("%s (%s, %d bytes)\n", keys[i], a.MimeType, a.Size)
		}
	}

	return nil
}

func runListImages(args []string) error {
	fs := flag.NewFlagSet("images", flag.ContinueOnError)
	inPath := fs.String("in", "", "input .epub path")
	compliance := fs.String("compliance", "flexible", "flexible|ebpaj|kadokawa")
	mode := fs.String("mode", "pages", "pages|all")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*inPath) == "" {
		return fmt.Errorf("-in is required")
	}

	f, size, err := openReaderAt(*inPath)
	if err != nil {
		return err
	}
	defer f.Close()

	level, err := parseCompliance(*compliance)
	if err != nil {
		return err
	}
	doc, err := epub.Decode(f, size, epub.WithCompliance(level))
	if err != nil {
		return err
	}

	type imageRow struct {
		Order    int    `json:"order,omitempty"`
		AssetID  string `json:"asset_id"`
		Href     string `json:"href"`
		MimeType string `json:"mime_type"`
		Size     uint64 `json:"size"`
		Checksum string `json:"checksum"`
		Width    int    `json:"width,omitempty"`
		Height   int    `json:"height,omitempty"`
		Spread   string `json:"spread,omitempty"`
	}

	rows := make([]imageRow, 0)
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "pages":
		refs, err := doc.ExtractReferencedImagesFromSpine()
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if ref.Page == nil || ref.Asset == nil {
				continue
			}
			rows = append(rows, imageRow{
				Order:    ref.Page.Order,
				AssetID:  ref.Asset.ID,
				Href:     ref.Href,
				MimeType: ref.Asset.MimeType,
				Size:     ref.Asset.Size,
				Checksum: ref.Asset.Checksum,
				Width:    ref.Page.Width,
				Height:   ref.Page.Height,
				Spread:   ref.Page.Spread,
			})
		}
	case "all":
		hrefs := make([]string, 0, len(doc.Assets))
		for href, a := range doc.Assets {
			if a != nil && strings.HasPrefix(a.MimeType, "image/") {
				hrefs = append(hrefs, href)
			}
		}
		sort.Strings(hrefs)
		for _, href := range hrefs {
			a := doc.Assets[href]
			rows = append(rows, imageRow{
				AssetID:  a.ID,
				Href:     href,
				MimeType: a.MimeType,
				Size:     a.Size,
				Checksum: a.Checksum,
			})
		}
	default:
		return fmt.Errorf("-mode must be pages or all")
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(*mode), "pages") {
			fmt.Printf("[%d] id=%s href=%s mime=%s size=%d checksum=%s spread=%s viewport=%dx%d\n",
				r.Order, r.AssetID, r.Href, r.MimeType, r.Size, r.Checksum, r.Spread, r.Width, r.Height)
			continue
		}
		fmt.Printf("id=%s href=%s mime=%s size=%d checksum=%s\n",
			r.AssetID, r.Href, r.MimeType, r.Size, r.Checksum)
	}
	return nil
}

func runBuildFromImages(args []string) error {
	fs := flag.NewFlagSet("build-images", flag.ContinueOnError)
	outPath := fs.String("out", "", "output .epub path")
	title := fs.String("title", "Untitled", "book title")
	compliance := fs.String("compliance", "flexible", "flexible|ebpaj|kadokawa")
	legacyTOC := fs.Bool("legacy-toc", false, "also generate EPUB 2 toc.ncx")
	direction := fs.String("direction", "rtl", "rtl|ltr")
	layout := fs.String("layout", "pre-paginated", "pre-paginated|reflowable")
	spread := fs.String("spread", "right", "left|right|center|none")
	globPattern := fs.String("glob", "", "glob pattern for images (e.g. ./images/*.jpg)")
	cover := fs.String("cover", "", "path to cover image (added as first page with cover semantics)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*outPath) == "" {
		return fmt.Errorf("-out is required")
	}

	level, err := parseCompliance(*compliance)
	if err != nil {
		return err
	}

	imagePaths := make([]string, 0)
	if strings.TrimSpace(*globPattern) != "" {
		matched, err := filepath.Glob(*globPattern)
		if err != nil {
			return err
		}
		sort.Strings(matched)
		imagePaths = append(imagePaths, matched...)
	}
	imagePaths = append(imagePaths, fs.Args()...)
	if len(imagePaths) == 0 {
		return fmt.Errorf("no images specified; use -glob and/or positional image paths")
	}

	layoutType, err := parseLayout(*layout)
	if err != nil {
		return err
	}
	directionNorm, err := parseDirection(*direction)
	if err != nil {
		return err
	}
	spreadNorm, err := normalizeSpread(*spread)
	if err != nil {
		return err
	}

	doc := &epub.Document{
		Metadata:  epub.Metadata{Title: strings.TrimSpace(*title)},
		Direction: directionNorm,
		Layout:    layoutType,
		Pages:     make([]*epub.Page, 0, len(imagePaths)),
		Assets:    make(map[string]*epub.Asset, len(imagePaths)),
	}

	openFiles := make([]*os.File, 0, len(imagePaths))
	defer func() {
		for _, f := range openFiles {
			_ = f.Close()
		}
	}()

	// Add cover image first if specified.
	if cp := strings.TrimSpace(*cover); cp != "" {
		cf, csize, err := openReaderAt(cp)
		if err != nil {
			return err
		}
		openFiles = append(openFiles, cf)
		if _, _, err := doc.SetCover(cf, csize); err != nil {
			return err
		}
	}

	for _, p := range imagePaths {
		f, size, err := openReaderAt(p)
		if err != nil {
			return err
		}
		openFiles = append(openFiles, f)

		if _, _, err := doc.AddPageWithAsset(f, size, spreadNorm); err != nil {
			return err
		}
	}

	outFile, err := os.Create(*outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encodeOpts := make([]epub.EncodeOption, 0, 1)
	if *legacyTOC {
		encodeOpts = append(encodeOpts, epub.WithLegacyTOC())
	}
	if err := epub.Encode(outFile, doc, encodeOpts...); err != nil {
		return err
	}

	if err := outFile.Close(); err != nil {
		return err
	}

	verifyFile, verifySize, err := openReaderAt(*outPath)
	if err != nil {
		return err
	}
	defer verifyFile.Close()
	if _, err := epub.Decode(verifyFile, verifySize, epub.WithCompliance(level)); err != nil {
		return fmt.Errorf("generated epub failed %s compliance validation: %w", strings.ToLower(strings.TrimSpace(*compliance)), err)
	}

	fmt.Printf("built: %s (%d images)\n", *outPath, len(imagePaths))
	return nil
}

func runRepack(args []string) error {
	fs := flag.NewFlagSet("repack", flag.ContinueOnError)
	inPath := fs.String("in", "", "input .epub path")
	outPath := fs.String("out", "", "output .epub path")
	compliance := fs.String("compliance", "flexible", "flexible|ebpaj|kadokawa")
	legacyTOC := fs.Bool("legacy-toc", false, "also generate EPUB 2 toc.ncx")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*inPath) == "" || strings.TrimSpace(*outPath) == "" {
		return fmt.Errorf("-in and -out are required")
	}

	inFile, size, err := openReaderAt(*inPath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	level, err := parseCompliance(*compliance)
	if err != nil {
		return err
	}
	doc, err := epub.Decode(inFile, size, epub.WithCompliance(level))
	if err != nil {
		return err
	}

	outFile, err := os.Create(*outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encodeOpts := make([]epub.EncodeOption, 0, 1)
	if *legacyTOC {
		encodeOpts = append(encodeOpts, epub.WithLegacyTOC())
	}
	if err := epub.Encode(outFile, doc, encodeOpts...); err != nil {
		return err
	}

	fmt.Printf("repacked: %s -> %s\n", *inPath, *outPath)
	return nil
}

func openReaderAt(path string) (*os.File, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, st.Size(), nil
}

func parseCompliance(s string) (epub.ComplianceLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "flexible":
		return epub.LevelFlexible, nil
	case "ebpaj":
		return epub.LevelEBPAJ, nil
	case "kadokawa":
		return epub.LevelKADOKAWA, nil
	default:
		return epub.LevelFlexible, fmt.Errorf("unknown compliance: %s", s)
	}
}

func parseLayout(s string) (epub.LayoutType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pre-paginated":
		return epub.LayoutPrePaginated, nil
	case "reflowable":
		return epub.LayoutReflowable, nil
	default:
		return epub.LayoutUnknown, fmt.Errorf("layout must be pre-paginated or reflowable")
	}
}

func parseDirection(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "rtl", "ltr":
		return strings.ToLower(strings.TrimSpace(s)), nil
	default:
		return "", fmt.Errorf("direction must be rtl or ltr")
	}
}

func normalizeSpread(s string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "left", "right", "center", "none":
		return v, nil
	default:
		return "", fmt.Errorf("spread must be left, right, center, or none")
	}
}

func printUsage() {
	_, _ = io.WriteString(os.Stderr, `epub command demo

Usage:
  epub inspect      -in book.epub [-compliance flexible|ebpaj|kadokawa]
	epub repack       -in book.epub -out out.epub [-compliance flexible|ebpaj|kadokawa] [-legacy-toc]
  epub images       -in book.epub [-mode pages|all] [-json] [-compliance flexible|ebpaj|kadokawa]
	epub build-images -out out.epub [-title TITLE] [-compliance flexible|ebpaj|kadokawa] [-legacy-toc] [-direction rtl|ltr] [-layout pre-paginated|reflowable]
					[-spread left|right|center|none] [-glob './images/*.jpg'] [image1 image2 ...]
`)
}
