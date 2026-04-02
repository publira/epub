# epub

An EPUB parsing and generation library built around `io.ReaderAt` and `io.Writer`.

## Features

- Filesystem-independent: `Decode` takes `io.ReaderAt` plus archive size
- Fail-fast: returns `DecodeError` immediately on structural violations
- Memory efficient: on-demand stream access via `Asset.Open`
- Strict validation: EBPAJ/KADOKAWA-style naming and directory checks

## Quick Start

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

You can also append pages with a shorthand helper:

```go
_, err = doc.AddPage(1600, 2560, "right")
if err != nil {
	log.Fatal(err)
}
```

And register assets without manually touching the map:

```go
img, err := os.Open("./item/image/p-001.jpg")
if err != nil {
	log.Fatal(err)
}
defer img.Close()

st, err := img.Stat()
if err != nil {
	log.Fatal(err)
}

_, _, err = doc.AddAsset("image/jpeg", img, st.Size())
if err != nil {
	log.Fatal(err)
}
```

For fixed-layout workflows, you can add both asset and page in one call:

```go
_, _, err = doc.AddPageWithAsset(
	img,
	st.Size(),
	"right",
)
if err != nil {
	log.Fatal(err)
}
```

## Demo CLI

The CLI lives under [cmd/epub](cmd/epub) and has its own documentation:

- [cmd/epub/README.md](cmd/epub/README.md)

Quick build:

```bash
go build -o ./bin/epub ./cmd/epub
```

## License

Apache License 2.0
