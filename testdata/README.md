# testdata

This directory holds sample EPUB files used by `testdata_test.go`.
The files are **not** committed to the repository because they are
copyrighted by their respective publishers.

## Obtaining the files

| File pattern                                                                | Source                                                                                                                  |
| --------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `kadokawa-*.epub`                                                           | [KADOKAWA EPUB Production Spec](https://kadokawa-epub.bookwalker.co.jp) ver.1.3.1 appendix                              |
| `book-template_*.epub`, `fixedlayout-template_*.epub`, `dpfj-sample_*.epub` | [DPFJ EPUB 3 Production Guide](https://dpfj.or.jp/counsel/guide) ver.1.1.4 appendix                                     |
| `jisx0213-check*.epub`                                                      | [KADOKAWA EPUB Production Spec](https://kadokawa-epub.bookwalker.co.jp) ver.1.3.1 appendix (character orientation test) |

Place the `.epub` files directly in this directory and run:

```
go test -run TestTestdata ./...
```
