# epub CLI

Practical command-line utility for working with EPUB files.

Source: [main.go](main.go)

## Build

```bash
go build -o ./bin/epub ./cmd/epub
```

## Commands

### inspect

Show document summary information.

```bash
./bin/epub inspect -in ./book.epub -compliance flexible
```

Options:

- `-in` input EPUB path (required)
- `-compliance` `flexible|ebpaj|kadokawa` (default: `flexible`)

### repack

Decode then encode again (round-trip smoke test).

```bash
./bin/epub repack -in ./book.epub -out ./book.repacked.epub -compliance flexible
```

Options:

- `-in` input EPUB path (required)
- `-out` output EPUB path (required)
- `-compliance` `flexible|ebpaj|kadokawa` (default: `flexible`)

### images

List image assets from an EPUB.

```bash
./bin/epub images -in ./book.epub -mode pages
```

JSON output:

```bash
./bin/epub images -in ./book.epub -mode all -json
```

Options:

- `-in` input EPUB path (required)
- `-mode` `pages|all` (default: `pages`)
- `-json` print JSON output
- `-compliance` `flexible|ebpaj|kadokawa` (default: `flexible`)

### build-images

Build an EPUB from image files.

```bash
./bin/epub build-images -out ./book.epub -title "My Book" -glob './images/*.jpg'
```

Strict generation validation:

```bash
./bin/epub build-images -out ./book.epub -title "My Book" -glob './images/*.jpg' -compliance ebpaj
```

Options:

- `-out` output EPUB path (required)
- `-title` document title (default: `Untitled`)
- `-compliance` `flexible|ebpaj|kadokawa` (default: `flexible`)
- `-direction` `rtl|ltr` (default: `rtl`)
- `-layout` `pre-paginated|reflowable` (default: `pre-paginated`)
- `-spread` `left|right|center|none` (default: `right`)
- `-glob` image glob pattern (optional)
- positional args: image paths (optional)

Note:

- You can combine `-glob` and positional image paths.
- Asset IDs and href paths are auto-generated in spec-friendly format (`p-000` and `item/image/p-000.ext`).
- `build-images` validates the generated EPUB using the selected `-compliance` level.
