# Security Audit Fixes (High + Medium) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the two High and four Medium findings from the 2026-11-21 security audit of `aws-go-tools`.

**Architecture:** The CLI stays a single `main.go`. Config loading and the RDS connection examples are pulled into small pure functions so they can be unit-tested without AWS. The workflow fixes are YAML-only.

**Tech Stack:** Go (Cobra, AWS SDK v2, `gopkg.in/yaml.v3`), GitHub Actions.

**Spec:** There is no spec file; the audit was delivered in conversation. The findings this plan implements are:

| ID | Finding | Location at time of audit |
|---|---|---|
| H1 | CI resolves Go from `go.mod` (`go 1.25.0`); `govulncheck` reports 37 reachable standard-library vulnerabilities at that version, 0 at Go 1.27.4 | `go.mod:3`, `release.yml:82-84`, `pr-validation.yml:22-24` |
| H2 | `config.yaml` in the working directory is read first and its `shell` value is sent unchecked as the SSM `command`; with `HOME` unset the home path is also relative | `main.go:173-186`, `main.go:385` |
| M1 | PR label names are interpolated into a `run:` script in a workflow with `contents: write` | `release.yml:34` |
| M2 | Third-party actions with write access are pinned to movable tags; permissions are workflow-wide | `release.yml:47`, `release.yml:113`, `pr-validation.yml:38` |
| M3 | Printed `mysql`/`psql` commands encrypt but do not verify the server certificate | `main.go:585`, `main.go:593` |
| M4 | The RDS token is printed and then repeated inside three copy-paste commands | `main.go:574`, `585`, `593`, `597` |

Low findings (L1-L7) are out of scope, except username quoting (L4), which Task 3 fixes because it rewrites those lines anyway.

## Global Constraints

- No new Go module dependencies.
- `gofmt -l .` prints nothing; `go vet ./...`, `go test -race ./...` and `golangci-lint run` pass with 0 issues after every task.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...` reports "No vulnerabilities found" after Task 1.
- Commit messages use `type: description` (`fix:`, `chore:`, `docs:`, `test:`), per `CONTRIBUTING.md`.
- Each task adds one line under "Unreleased" in `CHANGELOG.md`.
- Precondition: the working tree currently has uncommitted changes on branch `init` (lint fixes, CI changes, `CLAUDE.md`). Commit or stash those before Task 1 so each task's commit contains only its own change.
- New tests go in `main_test.go` (package `main`), table-driven with `t.Run`, matching the existing style.

## Review Focus

1. A config file that exists but is malformed or names an invalid shell: the tool exits with an error naming the file, and does not fall back to defaults or to the next path. (Task 2)
2. `HOME` unset: no relative path is ever searched; built-in defaults are used. (Task 2)
3. A shell value carrying arguments or metacharacters (`/bin/bash -c id`, `/bin/sh;id`, `$(id)`): rejected. (Task 2)
4. A Windows shell given as a full path with backslashes is accepted; one containing a space is rejected, and `CONFIG.md` says so. (Task 2)
5. A database username containing a single quote, and an engine that is neither MySQL-family nor PostgreSQL (e.g. `sqlserver-se`): the quote is escaped, and the unknown engine yields no command lines instead of a wrong one. (Task 3)

---

### Task 1: Raise the Go floor and gate CI on govulncheck (H1)

**Files:**
- Modify: `go.mod:3`, `go.sum`
- Modify: `.github/workflows/pr-validation.yml` (job `test`)
- Modify: `CLAUDE.md` (Commands block), `CHANGELOG.md`

- [ ] **Step 1: Confirm the failing state**

Run: `GOTOOLCHAIN=go1.25.0 go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./... | tail -5`
Expected: "Your code is affected by 37 vulnerabilities from the Go standard library."

- [ ] **Step 2: Raise the `go` directive**

Run: `go mod edit -go=1.27.4 && go mod tidy`
Expected: `go.mod` line 3 reads `go 1.27.4`; `golang.org/x/term` moves to the direct `require` block.

- [ ] **Step 3: Add a govulncheck step to the `test` job in `pr-validation.yml`, after "Run tests"**

```yaml
      - name: Run govulncheck
        run: go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...
```

- [ ] **Step 4: Verify**

Run: `go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./... | tail -8 && go test -race ./...`
Expected: "No vulnerabilities found." and `ok`.

- [ ] **Step 5: Document** — add the govulncheck command to the Commands block in `CLAUDE.md` with the comment `# CI runs this; fails on reachable vulnerabilities`, and a `CHANGELOG.md` line.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum .github/workflows/pr-validation.yml CLAUDE.md CHANGELOG.md
git commit -m "fix: build with Go 1.27.4 and run govulncheck in CI"
```

---

### Task 2: Harden config loading and validate the shell (H2)

**Files:**
- Modify: `main.go` — `main()`, `loadConfig` (currently lines 162-190), new `defaultConfigPaths`, `validateShell`
- Test: `main_test.go`
- Modify: `CONFIG.md`, `README.md` (any mention of a current-directory config), `CLAUDE.md` ("Shell selection comes from YAML config" bullet), `CHANGELOG.md`

**Interfaces:**
- Produces:
  - `func defaultConfigPaths() []string` — `<os.UserHomeDir()>/.aws-go-tools/config.yaml` (omitted when `UserHomeDir` errors or returns `""`), then `/etc/aws-go-tools/config.yaml`. Never a relative path.
  - `func loadConfig(explicitPath string, searchPaths []string) (Config, error)` — pure: no globals, no printing except one `fmt.Fprintf(os.Stderr, "Loaded configuration from: %s\n", path)`.
  - `func validateShell(platform, shell string) error` — `platform` is `"linux"` or `"windows"`.
  - New persistent flag `--config` / `-c` bound to global `configFile string`.

**Behaviour decided here:**
- `loadConfig` starts from the defaults (`/bin/bash`, `powershell.exe`).
- If `explicitPath != ""`: read only that file; a read, parse or validation failure is an error.
- Otherwise use the first path in `searchPaths` that exists. A path that does not exist is skipped; one that exists but fails to read, parse or validate is an error (fail closed; do not try the next path).
- Parse into a fresh `Config` seeded with defaults, so a failed parse can never leave a half-written value.
- Both shells in the result are validated before returning.
- `validateShell` patterns:
  - linux: `^/[A-Za-z0-9._/-]+$`
  - windows: `^([A-Za-z]:\\)?[A-Za-z0-9._\\-]+\.exe$`
  - Error text: `invalid <platform> shell %q: must be a bare executable path with no spaces or arguments`.
- `main()` stops calling `loadConfig()` at the top. The root command gets `PersistentPreRunE`, which calls `loadConfig(configFile, defaultConfigPaths())` and assigns the global `appConfig`; an error is returned to Cobra.
- The `os.ReadFile` line carries `// #nosec G304 G703 -- path is the --config flag or a fixed per-user/system location`.

- [ ] **Step 1: Write the failing tests**

```go
func TestValidateShell(t *testing.T) {
	tests := []struct {
		platform, shell string
		wantErr         bool
	}{
		{"linux", "/bin/bash", false},
		{"linux", "/usr/local/bin/fish", false},
		{"linux", "bash", true},
		{"linux", "/bin/bash -c id", true},
		{"linux", "/bin/sh;id", true},
		{"linux", "$(id)", true},
		{"linux", "", true},
		{"windows", "powershell.exe", false},
		{"windows", "cmd.exe", false},
		{"windows", `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, false},
		{"windows", `C:\Program Files\PowerShell\7\pwsh.exe`, true},
		{"windows", "cmd.exe /c calc.exe", true},
		{"windows", "powershell", true},
	}
	for _, tt := range tests {
		t.Run(tt.platform+"/"+tt.shell, func(t *testing.T) {
			if err := validateShell(tt.platform, tt.shell); (err != nil) != tt.wantErr {
				t.Errorf("validateShell(%q, %q) error = %v, wantErr %v", tt.platform, tt.shell, err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	write := func(t *testing.T, dir, body string) string {
		t.Helper()
		p := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	missing := filepath.Join(t.TempDir(), "absent.yaml")

	t.Run("no files gives defaults", func(t *testing.T) {
		cfg, err := loadConfig("", []string{missing})
		if err != nil || cfg.Linux.Shell != "/bin/bash" || cfg.Windows.Shell != "powershell.exe" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("working directory config is ignored", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "linux:\n  shell: /bin/zsh\n")
		t.Chdir(dir)
		cfg, err := loadConfig("", []string{missing})
		if err != nil || cfg.Linux.Shell != "/bin/bash" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("first existing search path wins and keeps other default", func(t *testing.T) {
		a := write(t, t.TempDir(), "linux:\n  shell: /bin/zsh\n")
		b := write(t, t.TempDir(), "linux:\n  shell: /bin/sh\n")
		cfg, err := loadConfig("", []string{missing, a, b})
		if err != nil || cfg.Linux.Shell != "/bin/zsh" || cfg.Windows.Shell != "powershell.exe" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("malformed file is an error, not a fallthrough", func(t *testing.T) {
		bad := write(t, t.TempDir(), "linux: [unclosed\n")
		good := write(t, t.TempDir(), "linux:\n  shell: /bin/sh\n")
		if _, err := loadConfig("", []string{bad, good}); err == nil || !strings.Contains(err.Error(), bad) {
			t.Fatalf("want error naming %s, got %v", bad, err)
		}
	})
	t.Run("invalid shell is an error", func(t *testing.T) {
		p := write(t, t.TempDir(), "linux:\n  shell: /bin/bash -c id\n")
		if _, err := loadConfig("", []string{p}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("explicit path is used and must exist", func(t *testing.T) {
		p := write(t, t.TempDir(), "windows:\n  shell: cmd.exe\n")
		cfg, err := loadConfig(p, []string{missing})
		if err != nil || cfg.Windows.Shell != "cmd.exe" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
		if _, err := loadConfig(missing, nil); err == nil {
			t.Fatal("want error for missing explicit path")
		}
	})
}

func TestDefaultConfigPaths(t *testing.T) {
	check := func(t *testing.T) {
		for _, p := range defaultConfigPaths() {
			if !filepath.IsAbs(p) {
				t.Errorf("relative config path %q", p)
			}
		}
	}
	t.Run("with HOME", func(t *testing.T) { t.Setenv("HOME", t.TempDir()); check(t) })
	t.Run("HOME unset", func(t *testing.T) { t.Setenv("HOME", ""); check(t) })
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test -run 'TestValidateShell|TestLoadConfig|TestDefaultConfigPaths' ./...`
Expected: build failure — `undefined: validateShell`, `undefined: defaultConfigPaths`, wrong argument count for `loadConfig`.

- [ ] **Step 3: Implement `defaultConfigPaths`, `validateShell`, the new `loadConfig`, the `--config` flag and `PersistentPreRunE` in `main.go`** as specified under "Behaviour decided here".

- [ ] **Step 4: Run the tests and the analyzers**

Run: `go test -race ./... && go vet ./... && golangci-lint run && gosec -quiet ./...`
Expected: tests `ok`; 0 lint issues; gosec no longer reports `main.go` `os.ReadFile` (G204 on `exec.Command` remains and is a known false positive).

- [ ] **Step 5: Manual check of the fix**

Run: `go build -o /tmp/agt . && (cd "$(mktemp -d)" && printf 'linux:\n  shell: /bin/sh;id\n' > config.yaml && /tmp/agt version; /tmp/agt --config ./config.yaml version)`
Expected: first call prints the version with no "Loaded configuration" line; second exits non-zero with `invalid linux shell`.

- [ ] **Step 6: Update docs**
  - `CONFIG.md`: locations become (1) `--config <path>`, (2) `~/.aws-go-tools/config.yaml`, (3) `/etc/aws-go-tools/config.yaml`; delete the "Project-Specific Configuration" section; state that the working directory is not searched and why; state the shell rules (absolute path on Linux, `.exe` name or path on Windows, no spaces or arguments); replace "the SSM session may fail" wording for invalid files with "the tool exits with an error".
  - `README.md`: add `--config/-c` to the options table; fix any text that says `config.yaml` is read from the current directory.
  - `CLAUDE.md`: rewrite the "Shell selection comes from YAML config" bullet to the new lookup order and validation; drop the sentence about a local `config.yaml` affecting `go run`.
  - `CHANGELOG.md`: one line, flagged as a behaviour change.

- [ ] **Step 7: Commit**

```bash
git add main.go main_test.go CONFIG.md README.md CLAUDE.md CHANGELOG.md
git commit -m "fix: stop reading config.yaml from the working directory and validate shell values"
```

---

### Task 3: Stop repeating the RDS token and verify the server certificate (M3, M4)

**Files:**
- Modify: `main.go` — `generateRDSAuthToken` (currently lines 546-610), new `shellQuote`, `rdsConnectionExamples`
- Test: `main_test.go`
- Modify: `README.md` ("RDS IAM Auth Token Generation" example output), `CHANGELOG.md`

**Interfaces:**
- Produces:
  - `func shellQuote(s string) string` — wraps in single quotes, replacing each `'` with `'\''`.
  - `func rdsConnectionExamples(engine, endpoint string, port int32, username string) []string` — output lines for the engine; empty slice for an engine that is neither MySQL-family (`mysql`, `mariadb`, `aurora-mysql`) nor PostgreSQL (`postgres`, `aurora-postgresql`). It does not take the token, so no example can contain it.

**Behaviour decided here:**
- `generateRDSAuthToken` prints the token exactly once (the existing block between the `=` rules). The `export PGPASSWORD='<token>'` line and every other token interpolation are removed.
- Before the examples it prints, verbatim:

```
Load the token into your shell without putting it in history:
  read -rs RDS_TOKEN && export RDS_TOKEN     # paste the token, press Enter
Download the RDS CA bundle once:
  curl -fsSLo global-bundle.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
```

- MySQL-family example line:
  `MYSQL_PWD="$RDS_TOKEN" mysql -h <endpoint> -P <port> -u <quoted user> --enable-cleartext-plugin --ssl-mode=VERIFY_IDENTITY --ssl-ca=./global-bundle.pem`
- PostgreSQL example line:
  `PGPASSWORD="$RDS_TOKEN" psql "host=<endpoint> port=<port> dbname=your_database user=<user> sslmode=verify-full sslrootcert=./global-bundle.pem"`
  Inside the psql conninfo string the username is not shell-quoted; if it contains anything outside `[A-Za-z0-9_.@-]`, emit the `-h/-p/-U <quoted user> -d your_database` form with `PGSSLMODE=verify-full PGSSLROOTCERT=./global-bundle.pem` instead.
- The "Notes" list changes "SSL/TLS connection is required" to "SSL/TLS with certificate verification is required; the examples verify against the RDS CA bundle".

- [ ] **Step 1: Write the failing tests**

```go
func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"admin":   `'admin'`,
		"o'brien": `'o'\''brien'`,
		"a b;id":  `'a b;id'`,
		"":        `''`,
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestRDSConnectionExamples(t *testing.T) {
	join := func(engine, user string) string {
		return strings.Join(rdsConnectionExamples(engine, "db.example.internal", 5432, user), "\n")
	}
	t.Run("postgres verifies the certificate", func(t *testing.T) {
		out := join("aurora-postgresql", "app")
		for _, want := range []string{"sslmode=verify-full", "sslrootcert=./global-bundle.pem", `PGPASSWORD="$RDS_TOKEN"`, "host=db.example.internal", "port=5432"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Contains(out, "sslmode=require") {
			t.Error("unverified sslmode present")
		}
	})
	t.Run("mysql verifies the certificate", func(t *testing.T) {
		out := join("mariadb", "app")
		for _, want := range []string{"--ssl-mode=VERIFY_IDENTITY", "--ssl-ca=./global-bundle.pem", `MYSQL_PWD="$RDS_TOKEN"`, "-u 'app'"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Contains(out, "--password") || strings.Contains(out, "REQUIRED") {
			t.Error("password argument or unverified ssl-mode present")
		}
	})
	t.Run("username with a quote is escaped", func(t *testing.T) {
		if out := join("mysql", "o'brien"); !strings.Contains(out, `'o'\''brien'`) {
			t.Errorf("unescaped username in:\n%s", out)
		}
		if out := join("postgres", "o'brien"); !strings.Contains(out, `-U 'o'\''brien'`) || !strings.Contains(out, "PGSSLMODE=verify-full") {
			t.Errorf("want -U form with quoted user, got:\n%s", out)
		}
	})
	t.Run("unknown engine yields no commands", func(t *testing.T) {
		if got := rdsConnectionExamples("sqlserver-se", "h", 1433, "sa"); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test -run 'TestShellQuote|TestRDSConnectionExamples' ./...`
Expected: build failure — `undefined: shellQuote`, `undefined: rdsConnectionExamples`.

- [ ] **Step 3: Implement `shellQuote` and `rdsConnectionExamples`, and rewrite the output section of `generateRDSAuthToken` to use them** per "Behaviour decided here".

- [ ] **Step 4: Run tests and confirm the token appears once in the source**

Run: `go test -race ./... && golangci-lint run && grep -n 'authToken' main.go`
Expected: `ok`; 0 issues; exactly two lines — the assignment from `auth.BuildAuthToken` and one `fmt.Println(authToken)`. No `Printf` takes it as an argument.

- [ ] **Step 5: Update `README.md`'s RDS example output to match the new text, and add a `CHANGELOG.md` line.**

- [ ] **Step 6: Commit**

```bash
git add main.go main_test.go README.md CHANGELOG.md
git commit -m "fix: print RDS token once and use certificate-verifying connection examples"
```

---

### Task 4: Remove the label script injection in the release workflow (M1)

**Files:**
- Modify: `.github/workflows/release.yml` — step "Determine version bump from PR labels" (currently lines 29-43)
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Replace the step with the env-based form**

```yaml
      - name: Determine version bump from PR labels
        id: determine_bump
        env:
          EVENT_NAME: ${{ github.event_name }}
          LABELS: ${{ join(github.event.pull_request.labels.*.name, ',') }}
        run: |
          BUMP_TYPE="patch"
          if [[ "$EVENT_NAME" == "pull_request" ]]; then
            echo "PR Labels: $LABELS"
            if [[ "$LABELS" == *"major"* ]] || [[ "$LABELS" == *"breaking"* ]]; then
              BUMP_TYPE="major"
            elif [[ "$LABELS" == *"minor"* ]] || [[ "$LABELS" == *"feature"* ]]; then
              BUMP_TYPE="minor"
            fi
          fi
          echo "bump_type=$BUMP_TYPE" >> "$GITHUB_OUTPUT"
          echo "Version bump type: $BUMP_TYPE"
```

- [ ] **Step 2: Verify no expression remains inside any `run:` block that carries PR-controlled data**

Run: `grep -n '\${{ *github\.event\.pull_request' .github/workflows/*.yml`
Expected: matches only on `env:` lines, `if:` lines, and the `github-script` free ones — none inside a `run: |` body. (`pr-validation.yml:113` uses `github.event.pull_request.number`, an integer; leave it.)

- [ ] **Step 3: Prove the script is inert against a hostile label**

Run: `EVENT_NAME=pull_request LABELS='x"; touch /tmp/pwned; "' GITHUB_OUTPUT=/dev/null bash -c "$(ruby -ryaml -e 'puts YAML.load_file(".github/workflows/release.yml")["jobs"]["bump-version"]["steps"][1]["run"]')"; ls /tmp/pwned`
Expected: prints `Version bump type: patch`; `ls` reports no such file.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml CHANGELOG.md
git commit -m "fix: pass PR labels to the release script through env"
```

---

### Task 5: Pin actions to commit SHAs, scope permissions per job, add Dependabot (M2)

**Files:**
- Modify: `.github/workflows/release.yml`, `.github/workflows/pr-validation.yml`
- Create: `.github/dependabot.yml`
- Modify: `CLAUDE.md` (Releases section), `CHANGELOG.md`

**Decisions:**
- Every `uses:` line in both workflows is pinned to a 40-character commit SHA with the version as a trailing comment, e.g. `uses: actions/checkout@<sha> # v4.x.y`.
- Versions to pin: the current latest release within the major already in use, except `softprops/action-gh-release`, which moves from `v1` to the latest `v2` (v1 runs on an end-of-life Node runtime; the inputs this workflow uses — `tag_name`, `name`, `body`, `files`, `draft`, `prerelease` — are unchanged in v2).
- Resolve each SHA with `gh api repos/<owner>/<repo>/commits/<tag> --jq .sha` and record the exact tag it came from in the comment. Do not copy SHAs from memory or from any other source.
- `golangci-lint`'s own `version: latest` input stays as it is (the user chose that; audit item L6 is out of scope).
- Permissions:
  - `release.yml`: workflow-level `permissions: { contents: read }`; job `bump-version` and job `create-release` each get `permissions: { contents: write }`; `build-and-release` inherits read.
  - `pr-validation.yml`: workflow-level `permissions: { contents: read }`; job `pr-comment` gets `permissions: { pull-requests: write }`.
- `.github/dependabot.yml`:

```yaml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
```

- [ ] **Step 1: Resolve and record the SHAs** for `actions/checkout`, `actions/setup-go`, `actions/upload-artifact`, `actions/download-artifact`, `actions/github-script`, `golangci/golangci-lint-action`, `anothrNick/github-tag-action`, `softprops/action-gh-release`.

- [ ] **Step 2: Rewrite every `uses:` line and the permissions blocks; create `.github/dependabot.yml`.**

- [ ] **Step 3: Verify**

Run: `grep -n 'uses:' .github/workflows/*.yml | grep -vE '@[0-9a-f]{40} # v?[0-9]' ; ruby -ryaml -e 'Dir[".github/**/*.yml"].each { |f| YAML.load_file(f) }; puts "yaml ok"'`
Expected: the `grep` prints nothing; `yaml ok`.

Run: `ruby -ryaml -e 'r=YAML.load_file(".github/workflows/release.yml"); p r["permissions"], r["jobs"].transform_values { |j| j["permissions"] }'`
Expected: `{"contents"=>"read"}` then `bump-version` and `create-release` showing `{"contents"=>"write"}`, `build-and-release` showing `nil`.

- [ ] **Step 4: Document** — in `CLAUDE.md`'s Releases section add: actions are pinned by SHA and updated by Dependabot; keep the `# vX.Y.Z` comment in step when changing a pin. Add a `CHANGELOG.md` line.

- [ ] **Step 5: Commit**

```bash
git add .github CLAUDE.md CHANGELOG.md
git commit -m "chore: pin actions to commit SHAs, scope workflow permissions per job, add dependabot"
```

- [ ] **Step 6: After the PR is opened, confirm in the Actions tab** that all four `pr-validation` jobs run and the PR comment is still posted. The release workflow cannot be exercised until merge; watch the first release run and confirm the tag, the five binaries and the release page.
