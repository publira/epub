// SPDX-License-Identifier: Apache-2.0

// Package epub provides stream-oriented decode and encode utilities for EPUB.
//
// The package is designed for server-side ingestion pipelines where filesystems
// may not be available or desirable. Decode accepts io.ReaderAt and size,
// enabling in-memory buffers, object storage downloads, and other random-access
// sources without introducing os.File dependencies in the core API.
//
// Key points:
//
//   - Fail-fast decoding with DecodeError path/rule context.
//   - Compliance modes for flexible, EBPAJ, and KADOKAWA-oriented checks.
//   - Stream-first asset model via Asset.Open to avoid large in-memory blobs.
//   - Helper methods on Document to auto-generate spec-friendly image IDs/paths.
//
// Basic decode example:
//
//	f, _ := os.Open("book.epub")
//	defer f.Close()
//	stat, _ := f.Stat()
//	doc, err := epub.Decode(f, stat.Size(), epub.WithCompliance(epub.LevelEBPAJ))
//	if err != nil {
//		// handle DecodeError
//	}
//	_ = doc
//
// Basic encode flow from image sources:
//
//	img, _ := os.Open("./p-001.jpg")
//	defer img.Close()
//	st, _ := img.Stat()
//	doc := &epub.Document{Title: "Demo", Direction: "rtl", Layout: epub.LayoutPrePaginated}
//	_, _, _ = doc.AddPageWithAsset(img, st.Size(), "right")
//	_ = epub.Encode(io.Discard, doc)
package epub
