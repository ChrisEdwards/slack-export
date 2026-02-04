# Slackdump Version Detection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Automatically use system slackdump when version >= 3.1.13, falling back to bundled binary for older versions.

**Architecture:** Add `SlackdumpVersion()` to parse version output, `CompareVersions()` for semver comparison, and update `FindSlackdump()` to check system PATH version before falling back to bundled. All new code goes in `internal/export/slackdump.go`.

**Tech Stack:** Go, os/exec for running slackdump, string parsing for version extraction

---

## Task 1: Add CompareVersions Function

**Files:**
- Modify: `internal/export/slackdump.go` (add function after line 17)
- Modify: `internal/export/slackdump_test.go` (add tests)

**Step 1: Write the failing tests for CompareVersions**

Add to `internal/export/slackdump_test.go`:

```go
func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		want    int
		wantErr bool
	}{
		{"equal", "3.1.13", "3.1.13", 0, false},
		{"a greater patch", "3.1.14", "3.1.13", 1, false},
		{"a less patch", "3.1.12", "3.1.13", -1, false},
		{"a greater minor", "3.2.0", "3.1.13", 1, false},
		{"a less minor", "3.0.0", "3.1.13", -1, false},
		{"a greater major", "4.0.0", "3.1.13", 1, false},
		{"a less major", "2.9.99", "3.1.13", -1, false},
		{"malformed a", "abc", "3.1.13", 0, true},
		{"malformed b", "3.1.13", "xyz", 0, true},
		{"empty a", "", "3.1.13", 0, true},
		{"empty b", "3.1.13", "", 0, true},
		{"missing patch", "3.1", "3.1.13", 0, true},
		{"too many segments", "3.1.13.4", "3.1.13", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersions(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareVersions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("CompareVersions() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/export -run TestCompareVersions`
Expected: FAIL with "undefined: CompareVersions"

**Step 3: Write minimal implementation**

Add to `internal/export/slackdump.go` after the imports (before `findSlackdumpInDir`):

```go
// MinSlackdumpVersion is the minimum version that has the bug fix from PR #444.
const MinSlackdumpVersion = "3.1.13"

// CompareVersions compares two semver strings (X.Y.Z format).
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// Returns error if either version is malformed.
func CompareVersions(a, b string) (int, error) {
	parseVersion := func(v string) ([3]int, error) {
		parts := strings.Split(v, ".")
		if len(parts) != 3 {
			return [3]int{}, fmt.Errorf("invalid version format: %q (expected X.Y.Z)", v)
		}
		var result [3]int
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return [3]int{}, fmt.Errorf("invalid version segment %q: %w", p, err)
			}
			result[i] = n
		}
		return result, nil
	}

	va, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	vb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}

	for i := 0; i < 3; i++ {
		if va[i] < vb[i] {
			return -1, nil
		}
		if va[i] > vb[i] {
			return 1, nil
		}
	}
	return 0, nil
}
```

Add `"strconv"` to the imports.

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/export -run TestCompareVersions`
Expected: PASS

**Step 5: Run full check**

Run: `make check-test`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/export/slackdump.go internal/export/slackdump_test.go
git commit -m "$(cat <<'EOF'
feat(export): add CompareVersions for semver comparison

Implement simple X.Y.Z version comparison for determining if system
slackdump meets minimum version requirements.
EOF
)"
```

---

## Task 2: Add SlackdumpVersion Function

**Files:**
- Modify: `internal/export/slackdump.go`
- Modify: `internal/export/slackdump_test.go`

**Step 1: Write the failing tests for SlackdumpVersion**

Add to `internal/export/slackdump_test.go`:

```go
func TestSlackdumpVersion(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		want       string
		wantErr    bool
		errContains string
	}{
		{
			name:   "valid version",
			output: "Slackdump 3.1.13 (commit: abc12345) built on: 2024-01-15",
			want:   "3.1.13",
		},
		{
			name:   "version with v prefix in output",
			output: "Slackdump 3.2.0 (commit: def67890) built on: 2024-02-01",
			want:   "3.2.0",
		},
		{
			name:        "unknown version",
			output:      "Slackdump unknown (commit: unknown) built on: unknown",
			wantErr:     true,
			errContains: "unknown",
		},
		{
			name:        "malformed output",
			output:      "not slackdump output",
			wantErr:     true,
			errContains: "parse",
		},
		{
			name:        "empty output",
			output:      "",
			wantErr:     true,
			errContains: "parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSlackdumpVersion(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSlackdumpVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if got != tt.want {
				t.Errorf("parseSlackdumpVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/export -run TestSlackdumpVersion`
Expected: FAIL with "undefined: parseSlackdumpVersion"

**Step 3: Write minimal implementation**

Add to `internal/export/slackdump.go` after `CompareVersions`:

```go
// parseSlackdumpVersion extracts the version from slackdump version output.
// Expected format: "Slackdump 3.1.13 (commit: abc12345) built on: 2024-01-15"
func parseSlackdumpVersion(output string) (string, error) {
	// Look for "Slackdump " prefix
	const prefix = "Slackdump "
	if !strings.HasPrefix(output, prefix) {
		return "", fmt.Errorf("failed to parse slackdump version: unexpected output format")
	}

	// Extract version between "Slackdump " and " ("
	rest := output[len(prefix):]
	idx := strings.Index(rest, " (")
	if idx == -1 {
		return "", fmt.Errorf("failed to parse slackdump version: missing version delimiter")
	}

	version := rest[:idx]
	if version == "unknown" {
		return "", fmt.Errorf("slackdump version is unknown (development build)")
	}

	return version, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/export -run TestSlackdumpVersion`
Expected: PASS

**Step 5: Add SlackdumpVersion function that runs the binary**

Add to `internal/export/slackdump.go` after `parseSlackdumpVersion`:

```go
// SlackdumpVersion runs slackdump version and returns the parsed version string.
func SlackdumpVersion(binaryPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// #nosec G204 -- binaryPath comes from FindSlackdump, not untrusted input
	cmd := exec.CommandContext(ctx, binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run slackdump version: %w", err)
	}

	return parseSlackdumpVersion(strings.TrimSpace(string(output)))
}
```

**Step 6: Run full check**

Run: `make check-test`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/export/slackdump.go internal/export/slackdump_test.go
git commit -m "$(cat <<'EOF'
feat(export): add SlackdumpVersion to parse version output

Parse slackdump version command output to extract semver string.
Rejects "unknown" versions from development builds.
EOF
)"
```

---

## Task 3: Update FindSlackdump to Check Version

**Files:**
- Modify: `internal/export/slackdump.go:38-63` (FindSlackdump function)
- Modify: `internal/export/slackdump_test.go`
- Modify: `internal/export/slackdump_bundled_test.go`

**Step 1: Write tests for new FindSlackdump behavior**

Add to `internal/export/slackdump_bundled_test.go`:

```go
func TestFindSlackdump_SystemVersionSufficient(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create a mock slackdump that reports version 3.2.0 (above minimum)
	pathDir := t.TempDir()
	mockBin := filepath.Join(pathDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump 3.2.0 (commit: test1234) built on: 2024-01-01"
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Create bundled binary (should NOT be used)
	bundledDir := t.TempDir()
	bundledBin := filepath.Join(bundledDir, "slackdump")
	if err := os.WriteFile(bundledBin, []byte("#!/bin/sh\necho bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	t.Setenv("PATH", pathDir)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != mockBin {
		t.Errorf("FindSlackdump() = %q, want system PATH %q (version sufficient)", got, mockBin)
	}
}

func TestFindSlackdump_SystemVersionInsufficient(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create a mock slackdump that reports version 3.1.12 (below minimum)
	pathDir := t.TempDir()
	mockBin := filepath.Join(pathDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump 3.1.12 (commit: old12345) built on: 2023-01-01"
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Create bundled binary (should be used as fallback)
	bundledDir := t.TempDir()
	bundledBin := filepath.Join(bundledDir, "slackdump")
	if err := os.WriteFile(bundledBin, []byte("#!/bin/sh\necho bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	t.Setenv("PATH", pathDir)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != bundledBin {
		t.Errorf("FindSlackdump() = %q, want bundled %q (system version too old)", got, bundledBin)
	}
}

func TestFindSlackdump_SystemVersionUnknown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create a mock slackdump that reports unknown version (dev build)
	pathDir := t.TempDir()
	mockBin := filepath.Join(pathDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump unknown (commit: unknown) built on: unknown"
`
	if err := os.WriteFile(mockBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Create bundled binary (should be used as fallback)
	bundledDir := t.TempDir()
	bundledBin := filepath.Join(bundledDir, "slackdump")
	if err := os.WriteFile(bundledBin, []byte("#!/bin/sh\necho bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	t.Setenv("PATH", pathDir)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != bundledBin {
		t.Errorf("FindSlackdump() = %q, want bundled %q (system version unknown)", got, bundledBin)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/export -run TestFindSlackdump_SystemVersion`
Expected: FAIL (current implementation doesn't check version)

**Step 3: Update FindSlackdump implementation**

Replace the `FindSlackdump` function in `internal/export/slackdump.go`:

```go
// FindSlackdump locates the slackdump binary.
// Priority order:
// 1. System PATH if version >= MinSlackdumpVersion
// 2. Bundled binary next to the executable
func FindSlackdump() (string, error) {
	// Try system PATH first, check version
	if path, err := exec.LookPath("slackdump"); err == nil {
		version, verr := SlackdumpVersion(path)
		if verr == nil {
			cmp, cerr := CompareVersions(version, MinSlackdumpVersion)
			if cerr == nil && cmp >= 0 {
				return path, nil
			}
			// Version is below minimum, fall back to bundled
			fmt.Printf("System slackdump version %s is below minimum %s, using bundled binary\n",
				version, MinSlackdumpVersion)
		}
		// Version check failed (unknown or parse error), fall back to bundled
	}

	// Try bundled binary
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

	return "", errors.New("slackdump not found - ensure it's installed alongside slack-export")
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/export -run TestFindSlackdump`
Expected: PASS

**Step 5: Run full check**

Run: `make check-test`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/export/slackdump.go internal/export/slackdump_bundled_test.go
git commit -m "$(cat <<'EOF'
feat(export): FindSlackdump checks system version before bundled

Update priority order:
1. System PATH if version >= 3.1.13
2. Bundled binary as fallback

This allows automatic transition to system slackdump once users
upgrade to a version containing the fix from PR #444.
EOF
)"
```

---

## Task 4: Update Existing Tests

**Files:**
- Modify: `internal/export/slackdump_test.go`
- Modify: `internal/export/slackdump_bundled_test.go`

**Step 1: Update TestFindSlackdump_FromPATH**

The existing test needs updating because now PATH lookup checks version. Update in `internal/export/slackdump_test.go`:

```go
func TestFindSlackdump_FromPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create a mock slackdump with sufficient version
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump 3.2.0 (commit: test1234) built on: 2024-01-01"
`
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	// Use empty exe dir so bundled lookup fails
	oldExeDir := testExeDir
	testExeDir = t.TempDir() // empty dir
	defer func() { testExeDir = oldExeDir }()

	// Prepend tmpDir to PATH
	t.Setenv("PATH", tmpDir)

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != fakeBin {
		t.Errorf("FindSlackdump() = %q, want %q", got, fakeBin)
	}
}
```

Add `"runtime"` to the imports if not already present.

**Step 2: Update TestFindSlackdump_PrefersBundledOverPATH**

This test's semantics have changed. Now PATH is checked first if version is sufficient. Update in `internal/export/slackdump_bundled_test.go`:

```go
func TestFindSlackdump_PrefersBundledOverPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	// Create PATH binary with OLD version (should NOT be used)
	pathDir := t.TempDir()
	pathBin := filepath.Join(pathDir, "slackdump")
	script := `#!/bin/sh
echo "Slackdump 3.0.0 (commit: old12345) built on: 2023-01-01"
`
	if err := os.WriteFile(pathBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	// Create bundled binary (should be used because PATH version is old)
	bundledDir := t.TempDir()
	bundledPath := filepath.Join(bundledDir, "slackdump")
	if err := os.WriteFile(bundledPath, []byte("#!/bin/sh\necho bundled"), 0755); err != nil {
		t.Fatal(err)
	}

	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != bundledPath {
		t.Errorf("FindSlackdump() = %q, want bundled %q (PATH version too old)", got, bundledPath)
	}
}
```

**Step 3: Update TestFindSlackdump_FallsBackToPATH**

Remove or update this test - the new behavior is different. The fallback is now to bundled, not PATH. Replace with:

```go
func TestFindSlackdump_FallsBackToBundled(t *testing.T) {
	// No PATH binary, should use bundled
	bundledDir := t.TempDir()
	binaryName := "slackdump"
	if runtime.GOOS == "windows" {
		binaryName = "slackdump.exe"
	}
	bundledPath := filepath.Join(bundledDir, binaryName)
	if err := os.WriteFile(bundledPath, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	oldExeDir := testExeDir
	testExeDir = bundledDir
	defer func() { testExeDir = oldExeDir }()

	// Empty PATH
	t.Setenv("PATH", t.TempDir())

	got, err := FindSlackdump()
	if err != nil {
		t.Fatalf("FindSlackdump() error = %v", err)
	}
	if got != bundledPath {
		t.Errorf("FindSlackdump() = %q, want bundled %q", got, bundledPath)
	}
}
```

**Step 4: Run full test suite**

Run: `make check-test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/export/slackdump_test.go internal/export/slackdump_bundled_test.go
git commit -m "$(cat <<'EOF'
test(export): update tests for new FindSlackdump priority order

Adjust tests to reflect new behavior:
- System PATH checked first with version verification
- Bundled binary used as fallback when version insufficient
EOF
)"
```

---

## Task 5: Final Verification and Cleanup

**Files:**
- None (verification only)

**Step 1: Run full test suite**

Run: `make check-test`
Expected: PASS with all tests

**Step 2: Verify no lint warnings**

Run: `make check VERBOSE=1`
Expected: All checks pass

**Step 3: Review git log**

Run: `git log --oneline -5`
Expected: See 4 commits for this feature

**Step 4: Update issue status**

Run: `br update se-2g5 --status in_progress`

---

## Summary of Changes

1. **`internal/export/slackdump.go`**:
   - Add `MinSlackdumpVersion` constant ("3.1.13")
   - Add `CompareVersions()` for semver comparison
   - Add `parseSlackdumpVersion()` to extract version from output
   - Add `SlackdumpVersion()` to run binary and get version
   - Update `FindSlackdump()` to check system version first

2. **`internal/export/slackdump_test.go`**:
   - Add `TestCompareVersions` table-driven tests
   - Add `TestSlackdumpVersion` for parsing tests
   - Update `TestFindSlackdump_FromPATH` for new behavior

3. **`internal/export/slackdump_bundled_test.go`**:
   - Add `TestFindSlackdump_SystemVersionSufficient`
   - Add `TestFindSlackdump_SystemVersionInsufficient`
   - Add `TestFindSlackdump_SystemVersionUnknown`
   - Update existing preference tests for new priority order
