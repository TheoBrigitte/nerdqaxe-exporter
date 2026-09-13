# Contributing

## Setup

Install the pre-commit hooks, which check formatting and commit messages:

```
make setup
```

## Build and run

```
make build                      # build into build/<os>/<arch>/bin/
make run TARGET=http://192.0.2.10
make install                    # copy the binary to ~/.local/bin
```

Cross compile with `make build-amd64` / `make build-arm64`, or set `GOOS` and
`GOARCH` on `make build`.

## Docker

```
make docker                     # build the image for the host platform
make docker-all                 # build for linux/amd64 and linux/arm64
make run-docker TARGET=http://192.0.2.10
```

## Test and lint

```
make test
make lint                       # golangci-lint, golint, go vet, gofmt
make security                   # nancy vulnerability scan
```

CI runs the same linters, tests with `-race`, and a GoReleaser snapshot build on
every push. `make help` lists all targets.

## Commit messages

Commits follow [conventional commits](https://www.conventionalcommits.org/);
the allowed types and scopes are in `git-conventional-commits.yaml`,
and the commit-msg hook enforces them.

## Releasing

Push a `v*` tag. CI runs GoReleaser, which publishes the binaries to a GitHub
release and the image to `docker.io/theo01/nerdqaxe-exporter`.
