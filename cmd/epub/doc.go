// SPDX-License-Identifier: Apache-2.0

// Command epub provides practical EPUB utilities for inspection, image listing,
// and image-based EPUB generation.
//
// Build:
//
//	go build -o ./bin/epub ./cmd/epub
//
// Examples:
//
//	./bin/epub inspect -in ./book.epub
//	./bin/epub images -in ./book.epub -mode pages
//	./bin/epub build-images -out ./book.epub -glob './images/*.jpg'
package main
