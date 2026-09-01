# epub

An EPUB parsing and generation library built around `io.ReaderAt` and `io.Writer`.

## Features

- Filesystem-independent: `Decode` takes `io.ReaderAt` plus archive size
- Fail-fast: returns `DecodeError` immediately on structural violations
- Memory efficient: on-demand stream access via `Asset.Open`
- Strict validation: EBPAJ/KADOKAWA-style naming and directory checks
- Fixed-layout image pages: SVG wrappers preserve page-fit across reading systems
- Structural semantics: cover, table of contents, bodymatter, index, and glossary landmarks

## Quick Start

Install:

```bash
go get github.com/publira/epub
```

```go
f, err := os.Open("book.epub")
if err != nil {
	log.Fatal(err)
}
defer f.Close()

st, err := f.Stat()
if err != nil {
	log.Fatal(err)
}

doc, err := epub.Decode(f, st.Size(), epub.WithCompliance(epub.LevelEBPAJ))
if err != nil {
	log.Fatal(err)
}

out, err := os.Create("out.epub")
if err != nil {
	log.Fatal(err)
}
defer out.Close()

if err := epub.Encode(out, doc); err != nil {
	log.Fatal(err)
}
```

For full API details and examples, see:

- https://pkg.go.dev/github.com/publira/epub

## Demo CLI

The CLI lives under [cmd/epub](cmd/epub) and has its own documentation:

- [cmd/epub/README.md](cmd/epub/README.md)

Quick build:

```bash
go build -o ./bin/epub ./cmd/epub
```

## EPUB specification validation

Validate a generated publication with the official W3C EPUBCheck tool:

```bash
./scripts/run-epubcheck.sh path/to/book.epub
```

The script requires Java 11 or newer and caches EPUBCheck under `.tools/`.
The development container includes Temurin Java 25.0.4.

## License

Apache License 2.0
