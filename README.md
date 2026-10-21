# traefik-syntax-check

This repository contains a tool to analyse the syntax from your v2 traefik deployment and help you validate which rules need to be update to match the v3 syntax.

## Old method

Before creating this tool I was using this command shell command:

### Retrieve Traefik configuration

```shell
curl -s https://traefik-dashboard.domain.com/api/rawdata | yq -P '{"http": (del(.. | select(has("using")).using) | del(.. | select(has("status")).status) | del(.. | select(has("serverStatus")).serverStatus)| del(.. | select(has("usedBy")).usedBy) | del(.. | select(has("ruleSyntax")).ruleSyntax) | .routers[].service |= (select(. != null) | select(test("@internal") |not) | . + "@docker@file" ))}' > ./validation/rawdata.yaml
```

### Download Traefik v3

```shell
curl -OL https://github.com/traefik/traefik/releases/download/v3.7.1/traefik_v3.7.1_darwin_arm64.tar.gz && tar -xzf traefik_v3.7.1_darwin_arm64.tar.gz traefik && rm traefik_v3.7.1_darwin_arm64.tar.gz && chmod +x traefik
```

### Start traefik v3 locally with extracted data

```shell
./traefik --api.dashboard=true --api.insecure=true --log.format=json --log.level=DEBUG --log.filePath=./traefik.log --providers.file.directory=./validation --entrypoints.name=web --entrypoints.web.address=":20080" --entrypoints.name=websecure --entrypoints.websecure.address=":20443"
```

### Export router list with error

```shell
grep "error while parsing rule" traefik.log | jq '.routerName + " " + .error' | sort | uniq
```

## New method

`tsc` (traefik-syntax-check) replaces the steps above with a single command.
It fetches the routers from the Traefik v2 API (`/api/rawdata`) and validates each HTTP and TCP router rule
with the Traefik v3 rule parsers, imported as a Go library.
No Traefik v3 binary, local instance or log parsing is needed.

The rules are validated against the Traefik version pinned in `go.mod` (currently v3.7.1, see `tsc -version`).

### Install

```shell
go install github.com/NEwa-05/traefik-syntax-check/cmd/tsc@latest
```

Or build it from a clone of this repository:

```shell
make build
```

### Usage

The Traefik v2 API must be enabled.
`-url` accepts the API root, the dashboard URL or the `/api/rawdata` endpoint itself.

```shell
tsc -url https://traefik-dashboard.domain.com
```

```text
Checked 7 router rule(s) against Traefik v3.7.1 syntax: 4 need(s) to be updated.

PROTOCOL  ROUTER                 RULE                                         ERROR
http      headers@file           Headers(`X-Env`, `prod`)                     unsupported function: Headers
http      methods@kubernetescrd  Method(`GET`, `POST`)                        error while adding rule Method: unexpected number of parameters; got 2, expected one of [1]
http      multi-host@docker      Host(`a.domain.com`, `b.domain.com`)         error while adding rule Host: unexpected number of parameters; got 2, expected one of [1]
tcp       postgres@file          HostSNI(`db1.domain.com`, `db2.domain.com`)  unexpected number of parameters; got 2, expected 1
```

Only the routers whose rule needs to be updated are listed, sorted by protocol then router name.
Routers from the `internal` provider are skipped, as Traefik v3 generates its own.

| Flag                | Description                                                                  |
|---------------------|------------------------------------------------------------------------------|
| `-url`              | Traefik v2 API or dashboard base URL                                         |
| `-file`             | Read the raw data from a JSON file saved from `/api/rawdata` instead of `-url` |
| `-user`             | Basic auth credentials as `user:password`                                    |
| `-header`           | Extra HTTP header as `"Name: value"`, repeatable                             |
| `-insecure`         | Skip TLS certificate verification                                            |
| `-timeout`          | HTTP request timeout (default `30s`)                                         |
| `-output`           | Output format: `text` (default) or `json`                                    |
| `-save-raw`         | Save the raw data fetched from `-url` to a file                              |
| `-include-internal` | Also check the routers from the `internal` provider                          |
| `-version`          | Print the Traefik v3 version used for the validation                         |

Exit codes: `0` all rules are valid, `1` some rules need to be updated, `2` error.
This makes `tsc` usable as a CI check.

### Limitations

`tsc` reports the rules that Traefik v3 refuses to load.
Some v2 rules are still valid v3 syntax but have a different meaning, and are not reported yet, for example:

- `HostRegexp` and `PathPrefix`/`Path` with `{name:regexp}` placeholders, read as plain regular expressions or literals in v3.
- `Query(`key=value`)`, read as a check of the key `key=value` in v3.

### Development

| Target           | Description                                                               |
|------------------|---------------------------------------------------------------------------|
| `make build`     | Build `tsc` for the current platform                                      |
| `make build-all` | Build `tsc` for Linux, macOS and Windows (amd64 and arm64) into `dist/`   |
| `make test`      | Run the tests with the race detector and coverage                         |
| `make lint`      | Run [golangci-lint](https://golangci-lint.run) v2, configured in `.github/.golangci.yml` |
| `make fmt`       | Format the code with the golangci-lint formatters                         |
| `make check`     | Run `lint` and `test`                                                     |
| `make clean`     | Remove the build outputs                                                  |

### Release

Pushing a `v*` tag runs the [release workflow](.github/workflows/release.yml): it lints and tests the code,
builds `tsc` for every platform with `make build-all`, and publishes the binaries with a `SHA256SUMS` file
to the GitHub release of that tag.

```shell
git tag v0.1.0 && git push origin v0.1.0
```

### Upgrading Traefik

Traefik's `replace` directives are not applied to modules depending on it, so they are mirrored in `go.mod`,
and its unpublished nested module `github.com/traefik/traefik/dynamic/ext` is copied in `third_party/traefik-dynamic-ext`.
Keep both in sync with the Traefik `go.mod` when upgrading:

```shell
go get github.com/traefik/traefik/v3@vX.Y.Z && go mod tidy
```

## Credits

`tsc` was developed with the help of [Claude Code](https://claude.com/claude-code), Anthropic's AI coding assistant,
which wrote the tool, its tests, the Makefile and the release workflow.
