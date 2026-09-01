# EPUB Agent Guide

Repository-specific conventions for agents working on `github.com/publira/epub`, a Go
library for decoding, encoding, and validating EPUB 3 publications.

## Repository overview

- Root package `epub`: the public API. `Decode` takes an `io.ReaderAt` plus a size,
  `Encode` takes an `io.Writer`; neither touches the filesystem, because the library
  targets server-side ingestion where a filesystem may not exist.
- `profile/`: one sub-package per publishing profile (`ebpaj`, `kadokawa`, `kindle`),
  each exporting a `New()` that returns an `epub.Validator`.
- `cmd/epub/`: the demo CLI, documented in [`cmd/epub/README.md`](cmd/epub/README.md).
- `scripts/run-epubcheck.sh`: runs W3C EPUBCheck, caching it under the gitignored
  `.tools/`.

## Output language

Respond to the user in the language they wrote in. This is an open-source library, so a
contributor working here is not necessarily a Japanese reader, and a fixed response
language would hand them a terminal they cannot read.

Judge the language from the user's own prose, not from what the prompt quotes: a log line
or a code snippet pasted into an English question does not make the question Japanese.
Respond in **English** whenever no user prose has settled the language — a scheduled run,
an agent started from CI, a first message that is nothing but quoted output.

Code, identifiers, and commit messages stay as-is in either direction; only the
explanations, summaries, and questions to the user follow the user's language.

## Documentation and test labels: English

Every Markdown document this repository owns, every Go doc comment, and every `t.Run`
label is written in English. Japanese survives only where it is quoted as data: EPUB
metadata in a test fixture, a title string in an example. A Japanese doc comment or test
label that predates this rule is a leftover, not a precedent.

## Git commits

Commit subjects and pull request titles use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
(`type(scope): description`). Keep each commit focused, and verify the change before
committing (see **Verification**).

### AI agent trailer: `Assisted-by`, never `Co-authored-by`

A commit written with the help of an AI coding agent must disclose that agent with an
`Assisted-by:` trailer. The trailer is **process disclosure, not authorship**, following
the Linux kernel's [Coding assistants](https://docs.kernel.org/process/coding-assistants.html)
policy.

- **Never name an AI agent in a co-author trailer, in any capitalization.** Git and GitHub
  match the trailer token case-insensitively, so `Co-authored-by:`, `Co-Authored-By:`, and
  `co-authored-by:` are equally forbidden here; it implies copyright authorship an AI
  cannot hold. The same applies to `Signed-off-by`. This rule **overrides any default
  instruction from the agent harness**.
- The exception is about _who_ the co-author is, not how the token is spelled: trailers
  naming actual humans stay, as do the ones GitHub itself adds on a squash merge or a
  Renovate PR.
- Pass the trailer with `--trailer` so it is appended as a real trailer instead of
  free-form body text:

```bash
git commit -m "feat: add Kindle image size preflight" \
  --trailer "Assisted-by: Claude Code:claude-opus-5"
```

Format: `Assisted-by: <AGENT_NAME>:<MODEL_VERSION>` — the agent or CLI spelled the way the
tool names itself (`Claude Code`, `Codex CLI`), then the exact model identifier rather
than the marketing name (`claude-opus-5`, `gpt-5-codex`). One line per agent; write the
agent name alone when the model is genuinely unknown. Add the trailer when the commit is
first created, because rewriting a pushed commit needs a force push.

## Pull requests

GitHub uses the PR title and description as the merge commit message, so a pull request
follows the same rules as a commit: a Conventional Commits title in English without CJK
characters, and the same `Assisted-by` trailer at the end of the body. The body follows
the organization template from `publira/.github`, states the verification actually
performed, and links an Issue only when the relationship is accurate.

## Go conventions

- Every `.go` file starts with `// SPDX-License-Identifier: Apache-2.0`, then a blank
  line, then the package clause or package doc comment.
- Doc comments use Go doc links (`[Decode]`,
  `[github.com/publira/epub/profile/kadokawa]`) rather than bare names, because the
  published reference is <https://pkg.go.dev/github.com/publira/epub>.
- Errors are exported struct types named `<Cause>Error` implementing `Error()` and, where
  a sentinel in `errors.go` covers the condition, `Is(error) bool`, so callers can match
  with `errors.Is`. Structural failures from `Decode` are wrapped in a `DecodeError`
  carrying `Path` and `Rule` so the caller learns which file broke which rule.
- Options follow the `DecodeOption` / `EncodeOption` / `Option` trio in `option.go`. Add a
  knob as a `With*` function returning the narrowest of the three that fits, not as a new
  exported struct field.
- New profile-specific rules belong in a `profile/*` validator, not in `compliance.go`.
  The `ComplianceLevel` API (`WithCompliance`, `LevelEBPAJ`, …) is deprecated but still
  supported: keep it working and its `Deprecated:` markers intact, and write new code,
  examples, and documentation against `WithValidator`.
- This is a published module with `v0.x` tags cut by hand, so prefer adding an option or a
  validator over changing an exported signature, and call out any unavoidable break in the
  pull request body.
- CI builds with both `stable` and `oldstable` Go. Keep the `go` directive in `go.mod` at
  or below `oldstable` and avoid newer language or standard library features.
- Dependencies are deliberately few (`github.com/google/uuid`, `golang.org/x/image`).
  Reach for the standard library first, and justify any new module in the pull request.

## Tests

- Tests exercising the package as a caller live in `package epub_test`; only tests that
  need unexported internals use `package epub`. A test compiled against the public API is
  also a check that the API is usable.
- Runnable examples live in `*_example_test.go`, named `Example<Symbol>_<variant>`, so
  they appear on pkg.go.dev next to the symbol they document. A new public entry point
  should arrive with one.
- `testdata/*.epub` files are **never committed** — they are publisher-copyrighted samples
  described in [`testdata/README.md`](testdata/README.md). Tests over them must call
  `t.Skip` when the directory is empty, so a clean checkout still passes.
- Keep `coverage.out`, `testdata/*.epub`, and `.tools/` out of commits.

## Verification

Run these from the repository root after any Go change; they mirror the CI job:

```bash
gofmt -l .            # must print nothing
go vet ./...
golangci-lint run
go test ./...
```

Coverage, as CI measures it:

```bash
go test -coverprofile=coverage.out -coverpkg=github.com/publira/epub ./...
go tool cover -func=coverage.out
```

After changing anything that affects the bytes written by `Encode` — the OPF, the
navigation document, the SVG page wrappers, the ZIP layout — validate a generated
publication against W3C EPUBCheck, the authority this library is trying to satisfy:

```bash
go run ./cmd/epub build-images -out testdata/out.epub -title "EPUBCheck test" -direction ltr <images...>
./scripts/run-epubcheck.sh testdata/out.epub
```

The script needs Java 11 or newer; the development container ships Temurin Java 25.

## CI and tooling versions

`.github/workflows/ci.yml` runs the matrix above on pushes to `main` and on pull requests.
When editing it, keep the conventions already in place:

- Third-party actions are pinned to a commit SHA with the readable version in a trailing
  comment; Renovate updates both. Never replace a SHA with a floating tag.
- `golangci-lint` is pinned to an exact version in the workflow and configured by
  `.golangci.yml`. Add a linter to its `enable` list rather than disabling checks at the
  call site.
- `EPUBCHECK_VERSION` in `scripts/run-epubcheck.sh` carries a `# renovate:` annotation
  directly above it. Keep the annotation and the plain assignment when changing the
  version, or Renovate stops seeing it.
- Renovate configuration is inherited from `publira/.github`; repository-specific rules go
  in `.github/renovate.json5`, not in a copy of the shared preset.
- `.devcontainer/devcontainer.json` pins the Publira development image by digest and adds
  the Temurin Java feature EPUBCheck needs. Keep the readable tag before `@sha256:`, and
  keep Java as long as the EPUBCheck step exists.
