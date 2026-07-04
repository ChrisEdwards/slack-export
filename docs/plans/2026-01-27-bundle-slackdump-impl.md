# Bundle Slackdump Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bundle the slackdump binary alongside slack-export in release archives, preferring the bundled binary at runtime.

**Architecture:** Modify `FindSlackdump()` to first check for a bundled binary next to the executable, then fall back to PATH. Remove the `configPath` parameter and `SlackdumpPath` config option since they're no longer needed. Create a GitHub Actions release workflow that builds both binaries for all platforms.

**Tech Stack:** Go, GitHub Actions, goreleaser-style cross-compilation

---

## Task 1: Update FindSlackdump Function Signature

**Files:**
- Modify: `internal/export/slackdump.go:18-33`
- Modify: `internal/export/slackdump_test.go` (multiple test functions)

**Step 1: Write the failing test for bundled binary detection**

Add a new test file `internal/export/slackdump_bundled_test.go`:

```go
package export

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindSlackdump_BundledBinary(t *testing.T) {
	// Create a fake "executable" directory with bundled slackdump
	tmpDir := t.TempDir()

	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}

	bundledPath := filepath.Join(tmpDir, binaryName)
	if err := os.WriteFile(bundledPath, []byte("fake bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	// Override executable path for test
	got, err := findSlackdumpInDir(tmpDir)
	if err != nil {
		t.Fatalf("findSlackdumpInDir() error = %v", err)
	}
	if got != bundledPath {
		t.Errorf("findSlackdumpInDir() = %q, want %q", got, bundledPath)
	}
}

func TestFindSlackdump_BundledBinaryNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// No bundled binary, should return error
	_, err := findSlackdumpInDir(tmpDir)
	if err == nil {
		t.Fatal("findSlackdumpInDir() with no bundled binary should return error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/export -run TestFindSlackdump_Bundled`
Expected: FAIL with "undefined: findSlackdumpInDir"

**Step 3: Implement helper function for bundled binary detection**

In `internal/export/slackdump.go`, add after the imports (around line 17):

```go
// findSlackdumpInDir looks for a slackdump binary in the given directory.
// Returns the full path if found, error otherwise.
func findSlackdumpInDir(dir string) (string, error) {
	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}

	bundled := filepath.Join(dir, binaryName)
	if _, err := os.Stat(bundled); err == nil {
		return bundled, nil
	}
	return "", fmt.Errorf("slackdump not found in %s", dir)
}
```

Add `"runtime"` to imports.

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/export -run TestFindSlackdump_Bundled`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/export/slackdump.go internal/export/slackdump_bundled_test.go
git commit -m "$(cat <<'EOF'
feat(export): add helper to find bundled slackdump binary

Prepare for bundled slackdump support by adding findSlackdumpInDir
helper that locates slackdump next to the main executable.
EOF
)"
```

---

## Task 2: Update FindSlackdump to Use Bundled Binary First

**Files:**
- Modify: `internal/export/slackdump.go:18-33`
- Modify: `internal/export/slackdump_test.go` (update existing tests)

**Step 1: Write test for new FindSlackdump behavior**

Update `internal/export/slackdump_bundled_test.go` to add:

```go
func TestFindSlackdump_PrefersBundledOverPATH(t *testing.T) {
	// Create bundled binary
	bundledDir := t.TempDir()
	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}
	bundledPath := filepath.Join(bundledDir, binaryName)
	if err := os.WriteFile(bundledPath, []byte("bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create PATH binary
	pathDir := t.TempDir()
	pathBin := filepath.Join(pathDir, binaryName)
	if err := os.WriteFile(pathBin, []byte("path"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	// Set the executable directory override for testing
	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != bundledPath {
		t.Errorf("FindSlackdump() = %q, want bundled %q (not PATH)", got, bundledPath)
	}
}

func TestFindSlackdump_FallsBackToPATH(t *testing.T) {
	// Empty bundled directory
	bundledDir := t.TempDir()

	// Create PATH binary
	pathDir := t.TempDir()
	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}
	pathBin := filepath.Join(pathDir, binaryName)
	if err := os.WriteFile(pathBin, []byte("path"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	// Set the executable directory override for testing
	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != pathBin {
		t.Errorf("FindSlackdump() = %q, want PATH %q", got, pathBin)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/export -run TestFindSlackdump_Prefers`
Expected: FAIL (signature change needed)

**Step 3: Update FindSlackdump signature and implementation**

Replace the existing `FindSlackdump` function in `internal/export/slackdump.go`:

```go
// testExeDir is used in tests to override os.Executable() directory.
// Empty string means use the real executable directory.
var testExeDir string

// FindSlackdump locates the slackdump binary.
// Priority order:
// 1. Bundled binary next to the executable
// 2. System PATH (fallback for development)
func FindSlackdump() (string, error) {
	// Try bundled binary first
	var exeDir string
	if testExeDir != "" {
		exeDir = testExeDir
	} else if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}

	if exeDir != "" {
		if path, err := findSlackdumpInDir(exeDir); err == nil {
			return path, nil
		}
	}

	// Fall back to PATH
	path, err := exec.LookPath("slackdump")
	if err != nil {
		return "", errors.New("slackdump not found - ensure it's installed alongside slack-export")
	}
	return path, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/export -run TestFindSlackdump`
Expected: FAIL (old tests use the old signature)

**Step 5: Commit partial progress**

```bash
git add internal/export/slackdump.go internal/export/slackdump_bundled_test.go
git commit -m "$(cat <<'EOF'
feat(export): FindSlackdump prefers bundled binary over PATH

Change FindSlackdump to check for bundled binary next to executable
before falling back to PATH lookup. Removes configPath parameter.
EOF
)"
```

---

## Task 3: Update Existing Tests for New Signature

**Files:**
- Modify: `internal/export/slackdump_test.go`

**Step 1: Update existing test functions**

Replace the existing `FindSlackdump` tests in `slackdump_test.go`:

```go
func TestFindSlackdump_FromPATH(t *testing.T) {
	// Create a temp dir with a fake slackdump binary
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "slackdump")
	if err := os.WriteFile(fakeBin, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	// Use empty exe dir so it falls back to PATH
	oldExeDir := testExeDir
	testExeDir = t.TempDir() // empty dir
	defer func() { testExeDir = oldExeDir }()

	// Prepend tmpDir to PATH
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != fakeBin {
		t.Errorf("FindSlackdump() = %q, want %q", got, fakeBin)
	}
}

func TestFindSlackdump_NotFound(t *testing.T) {
	// Set both exe dir and PATH to empty dirs
	oldExeDir := testExeDir
	testExeDir = t.TempDir()
	defer func() { testExeDir = oldExeDir }()

	t.Setenv("PATH", t.TempDir())

	_, err := FindSlackdump()
	if err == nil {
		t.Fatal("FindSlackdump() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should mention 'not found'", err.Error())
	}
}
```

Remove these old test functions that no longer apply:
- `TestFindSlackdump_ExplicitPath`
- `TestFindSlackdump_ExplicitPathNotFound`
- `TestFindSlackdump_NotInPATH`

**Step 2: Run all tests to verify they pass**

Run: `go test -v ./internal/export`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/export/slackdump_test.go
git commit -m "$(cat <<'EOF'
test(export): update tests for new FindSlackdump signature

Remove tests for obsolete configPath parameter, add tests for
bundled binary preference behavior.
EOF
)"
```

---

## Task 4: Update Call Sites in main.go

**Files:**
- Modify: `cmd/slack-export/main.go` (lines 362, 472)

**Step 1: Update initStepSlackdump call site**

In `cmd/slack-export/main.go`, change line 362 from:
```go
path, err := export.FindSlackdump("")
```
to:
```go
path, err := export.FindSlackdump()
```

**Step 2: Update initStepAuth call site**

In `cmd/slack-export/main.go`, change line 472 from:
```go
slackdumpPath, err := export.FindSlackdump("")
```
to:
```go
slackdumpPath, err := export.FindSlackdump()
```

**Step 3: Run linter to verify**

Run: `make check`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/slack-export/main.go
git commit -m "$(cat <<'EOF'
refactor(cmd): update FindSlackdump calls for new signature

Remove empty string arguments from FindSlackdump calls.
EOF
)"
```

---

## Task 5: Update Exporter Call Site

**Files:**
- Modify: `internal/export/exporter.go` (line 38)

**Step 1: Update NewExporter call**

In `internal/export/exporter.go`, change line 38 from:
```go
sdPath, err := FindSlackdump(cfg.SlackdumpPath)
```
to:
```go
sdPath, err := FindSlackdump()
```

**Step 2: Run tests**

Run: `make check-test`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/export/exporter.go
git commit -m "$(cat <<'EOF'
refactor(export): update exporter to use new FindSlackdump

Remove cfg.SlackdumpPath argument from FindSlackdump call.
EOF
)"
```

---

## Task 6: Remove SlackdumpPath from Config

**Files:**
- Modify: `internal/config/config.go` (line 20)
- Modify: `internal/config/config_test.go` (if tests reference it)

**Step 1: Remove SlackdumpPath field**

In `internal/config/config.go`, remove line 20:
```go
SlackdumpPath string   `yaml:"slackdump_path" mapstructure:"slackdump_path"`
```

**Step 2: Check for any config tests that need updating**

Run: `rg SlackdumpPath`
Update any test files that reference this field.

**Step 3: Run tests**

Run: `make check-test`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "$(cat <<'EOF'
refactor(config): remove obsolete SlackdumpPath option

The bundled binary approach removes the need for user configuration
of the slackdump path.
EOF
)"
```

---

## Task 7: Create GitHub Actions Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Create workflows directory**

```bash
mkdir -p .github/workflows
```

**Step 2: Write the release workflow**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: darwin
            goarch: arm64
            ext: ""
          - goos: darwin
            goarch: amd64
            ext: ""
          - goos: linux
            goarch: amd64
            ext: ""
          - goos: linux
            goarch: arm64
            ext: ""
          - goos: windows
            goarch: amd64
            ext: ".exe"

    steps:
      - name: Checkout slack-export
        uses: actions/checkout@v4

      - name: Checkout slackdump fork
        uses: actions/checkout@v4
        with:
          repository: chrisedwards/slackdump
          path: slackdump-fork
          ref: bugfix-branch  # Update this to your bug fix branch

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Build slack-export
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          go build -ldflags="-s -w" -o dist/slack-export${{ matrix.ext }} ./cmd/slack-export

      - name: Build slackdump
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          cd slackdump-fork
          go build -ldflags="-s -w" -o ../dist/slackdump${{ matrix.ext }} ./cmd/slackdump

      - name: Create archive (Unix)
        if: matrix.goos != 'windows'
        run: |
          cd dist
          tar -czvf slack-export-${{ github.ref_name }}-${{ matrix.goos }}-${{ matrix.goarch }}.tar.gz slack-export slackdump

      - name: Create archive (Windows)
        if: matrix.goos == 'windows'
        run: |
          cd dist
          zip slack-export-${{ github.ref_name }}-${{ matrix.goos }}-${{ matrix.goarch }}.zip slack-export.exe slackdump.exe

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: slack-export-${{ matrix.goos }}-${{ matrix.goarch }}
          path: dist/slack-export-*

  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: artifacts
          merge-multiple: true

      - name: List artifacts
        run: ls -la artifacts/

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: artifacts/*
          generate_release_notes: true
```

**Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "$(cat <<'EOF'
ci: add release workflow for bundled binaries

Build both slack-export and slackdump for all platforms on tag push.
Creates .tar.gz archives for Unix and .zip for Windows.
EOF
)"
```

---

## Task 8: Test Release Workflow Locally (Optional)

**Files:**
- None (manual verification)

**Step 1: Test local cross-compilation**

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/slack-export-linux ./cmd/slack-export
GOOS=darwin GOARCH=arm64 go build -o /tmp/slack-export-darwin ./cmd/slack-export
```

Expected: Both commands succeed without errors

**Step 2: Verify bundled detection works in development**

```bash
go build -o /tmp/testdir/slack-export ./cmd/slack-export
touch /tmp/testdir/slackdump && chmod +x /tmp/testdir/slackdump
/tmp/testdir/slack-export --help
```

The binary should find the slackdump next to it (you can verify by checking the init step behavior).

---

## Task 9: Update Documentation

**Files:**
- Modify: `docs/plans/2026-01-27-bundle-slackdump-design.md` (mark completed items)

**Step 1: Update the implementation checklist**

Mark completed items in the design doc:

```markdown
## Implementation Checklist

**Prerequisites (manual):**
- [ ] Fork slackdump to GitHub (e.g., chrisedwards/slackdump)
- [ ] Push bug fix branch to the fork

**Code changes:**
- [x] Update `FindSlackdump()` - remove configPath, add bundled-first logic
- [x] Update all call sites of `FindSlackdump`
- [x] Remove any config options for slackdump path
- [x] Add `.exe` suffix handling for Windows

**Release infrastructure:**
- [x] Create `.github/workflows/release.yml`
- [x] Build matrix for all 5 platforms
- [x] Package as .tar.gz (unix) / .zip (windows)
- [x] Create GitHub release on tag push

**Testing:**
- [x] Bundled binary detection works
- [x] PATH fallback works for development
- [ ] Release workflow succeeds with test tag
```

**Step 2: Commit**

```bash
git add docs/plans/2026-01-27-bundle-slackdump-design.md
git commit -m "$(cat <<'EOF'
docs: update bundling design with completed checklist

Mark implementation tasks as complete.
EOF
)"
```

---

## Summary of Changes

1. **`internal/export/slackdump.go`**: New `FindSlackdump()` signature (no params), adds `findSlackdumpInDir()` helper, Windows `.exe` suffix handling
2. **`internal/export/slackdump_test.go`**: Updated tests for new signature, removed obsolete tests
3. **`internal/export/slackdump_bundled_test.go`**: New tests for bundled binary detection
4. **`internal/export/exporter.go`**: Updated `FindSlackdump()` call
5. **`cmd/slack-export/main.go`**: Updated two `FindSlackdump()` calls
6. **`internal/config/config.go`**: Removed `SlackdumpPath` field
7. **`.github/workflows/release.yml`**: New release workflow

## Prerequisites Not Covered in This Plan

- Fork slackdump repository
- Create/identify the bug fix branch in the fork
- Update the workflow with the correct branch name
