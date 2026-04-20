# Contributing

## About this project

`llm-chat` is a study project I built to learn Go TUI development with Bubble Tea. It is open to contributions because I want to learn from review, ideas, and other perspectives.

## Before opening a PR

For non-trivial changes (new features, refactors, behavior changes), please open an issue first to discuss the approach. Small fixes — typos, obvious bugs, doc corrections — can go straight to a PR.

## Code standards

- `gofmt` and `go vet` must pass. CI enforces both.
- Follow [Conventional Commits](https://www.conventionalcommits.org/) for commit messages (`feat:`, `fix:`, `refactor:`, etc.).
- Keep commits atomic — one logical change per commit.

## Building and testing locally

```
make build       # compile to bin/llm-chat
make run         # run via go run
go test ./...    # run the test suite
go vet ./...     # static checks
gofmt -l .       # list any unformatted files (should print nothing)
```
