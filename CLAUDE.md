# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`aws-go-tools` is a single-binary Go CLI (Cobra) with three subcommands, all sharing the persistent flags `--profile/-p`, `--region/-r` and `--config/-c`:

- `ec2` — list EC2 instances, pick one interactively, open an SSM shell session
- `rds` — list RDS instances, pick one, print an IAM auth token plus engine-specific connection examples
- `eni <ip>` — find the ENI owning a private or public IP and identify the AWS service behind it
- `version` — prints `main.Version`

## Commands

```bash
make build                      # build ./aws-go-tools (VERSION=x.y.z to stamp a version)
make build-all                  # cross-compile linux/darwin/windows into build/
make test                       # go test -v ./...
go test -v -run TestRDSEngineFiltering ./...            # single test
go test -v -run 'TestInstanceStruct/<subtest name>' ./...   # single table-driven subtest
go test -v -race -coverprofile=coverage.out ./...       # what CI runs
make fmt                        # go fmt ./...
go vet ./...                    # CI runs this; there is no make target for it
go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...   # CI runs this; fails on reachable vulnerabilities
make lint                       # golangci-lint run (skipped with a message if not installed)
make deps                       # go mod download && go mod tidy
go run main.go ec2 --profile <profile> [--region <region>]
```

The version is injected at build time with `-ldflags "-X 'main.Version=...'"`; a plain `go build` reports `dev`.

CI (`.github/workflows/pr-validation.yml`) fails a PR on `gofmt -l .` output, `go vet`, test failures, reachable vulnerabilities reported by `govulncheck`, or any of the five cross-compile targets (built with `CGO_ENABLED=0`). golangci-lint runs with `continue-on-error`, so it is advisory only.

## Architecture

Everything lives in `main.go` (package `main`); there are no internal packages. Each subcommand follows the same pipeline:

1. The root command's `PersistentPreRunE` calls `loadConfig` and fills the global `appConfig` (so a bad config fails every subcommand, including `version`).
2. The Cobra `Run` func calls `loadAWSConfig(ctx)`, which reads the global `profile`/`region` flag vars into an `aws.Config`.
3. A `handle*` function orchestrates: `list*` (AWS SDK call → local struct) → `select*` (survey prompt) → action.

Things worth knowing before changing it:

- **Error handling is split by layer.** `list*`, `select*`, `connectToInstance`, `generateRDSAuthToken`, and `findENIByIP` return wrapped errors; only the `handle*` functions and `loadAWSConfig` call `log.Fatalf`. Keep new logic in the error-returning layer.
- **The SSM session is delegated to an external binary.** `connectToInstance` calls `ssm.StartSession` with the `AWS-StartInteractiveCommand` document (the shell is passed as its `command` parameter), then execs `session-manager-plugin` with the JSON-marshalled `SessionData`, the region, and `StartSession`. Before running it, the local terminal is put in raw mode and SIGINT is ignored in the parent so Ctrl+C reaches the remote shell as a byte instead of killing the session; both are restored via `defer`. Changes here need a manual test against a real instance.
- **Shell selection comes from YAML config.** `loadConfig(explicitPath, searchPaths)` uses the `--config/-c` file if given, otherwise the first existing of `~/.aws-go-tools/config.yaml` and, except on Windows, `/etc/aws-go-tools/config.yaml` (`defaultConfigPaths()`), and falls back to `/bin/bash` (Linux) and `powershell.exe` (Windows). The working directory is deliberately never searched, and every shell value must pass `validateShell` (bare executable path, no spaces or arguments), because the shell is the command SSM runs on the instance; a file that exists but is unreadable, malformed or invalid is a hard error, not a fallback. Platform is decided in `listInstances` from the EC2 `Platform` and `PlatformDetails` fields, defaulting to Linux. User-facing docs are in `CONFIG.md`.
- **Selection maps back by formatted string.** `selectInstance` / `selectRDSInstance` build option strings and look up the chosen one by string equality, so two entries that format identically resolve to the first.
- **Listing is not paginated.** `DescribeInstances` and `DescribeDBInstances` are each called once. Terminated EC2 instances and Oracle RDS engines (no IAM auth) are filtered out.
- **ENI service identification is heuristic.** For requester-managed ENIs, `identifyAWSService` substring-matches the description and requester ID against a `map`; map iteration order is random, so overlapping patterns (e.g. `elb` and `elbv2`) can match non-deterministically. Otherwise the attachment's instance ID or the ENI `InterfaceType` is used. VPC/subnet/instance name lookups are best-effort and swallow errors.

`main_test.go` only covers struct construction, the Oracle filter, and string/value sanity checks — nothing calls AWS, and there are no mocks or interfaces around the SDK clients.

## Releases

Merging a PR into `main` triggers `.github/workflows/release.yml`, which tags, cross-compiles, and publishes a GitHub release automatically. The version bump comes from PR labels: `major`/`breaking` → major, `minor`/`feature` → minor, anything else → patch. `CONTRIBUTING.md` asks for an entry under "Unreleased" in `CHANGELOG.md` and `type: description` commit messages (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`).

Every action in both workflows is pinned to a full commit SHA with the version in a trailing comment (`uses: owner/action@<sha> # v1.2.3`); Dependabot (`.github/dependabot.yml`) raises the update PRs. When changing a pin by hand, resolve the SHA with `gh api repos/<owner>/<repo>/commits/<tag> --jq .sha` and update the comment with it. Workflow-level permissions are read-only; only `bump-version`, `create-release` (`contents: write`) and `pr-comment` (`pull-requests: write`) get more.

Both workflows take their Go version from `go.mod` (`go-version-file`), so bumping the `go` directive there is the only change needed to move CI to a new Go release.
