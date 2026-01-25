# CI/CD Quick Reference Guide

## Active Workflows

### Fast Feedback CI (Primary)
**File:** `.github/workflows/pr-fast-feedback.yml`

**When it runs:**
- Every pull request with Go code changes
- Every push to main/develop with Go code changes
- Manual trigger
- Skips docs-only changes

**What it does:**
```
validate (lint) ──┬→ test-linux ───┐
                  ├→ test-windows ─┤→ all-checks-passed ✅
                  └→ build-check ──┘
```

**Time:** ~3-5 minutes
**Cost:** ~$0.04

---

## Running CI Checks Locally

### Lint
```bash
golangci-lint run ./...
```

### Unit Tests
```bash
# Linux (with race detection)
go test -race -v -count=1 -coverprofile=coverage.out ./...

# Windows (without race detection)
go test -v -count=1 ./...
```

### Build
```bash
go build -o immich-go main.go
```

### All Checks
```bash
golangci-lint run ./... && \
go test -race -v -count=1 ./... && \
go build -o immich-go main.go && \
echo "✅ All checks passed!"
```

---

## Manual Workflow Triggers

### Trigger Fast Feedback CI
```bash
# Via GitHub UI:
# 1. Go to Actions tab
# 2. Select "Fast Feedback CI"
# 3. Click "Run workflow"
# 4. Select branch
# 5. Click "Run workflow"
```

---

## Understanding CI Results

### Fast Feedback CI

**✅ All jobs passed:**
- Your code is ready for review
- No action needed

**❌ validate failed:**
- Linting issues detected
- Run `golangci-lint run ./...` locally
- Fix issues and push

**❌ test-linux or test-windows failed:**
- Unit tests failed
- Run `go test -v ./...` locally
- Fix failing tests and push

**❌ build-check failed:**
- Code doesn't compile
- Run `go build main.go` locally
- Fix compilation errors and push

---

## Path Filtering Rules

### Fast Feedback CI runs when:
```yaml
Changed files match:
  - **.go          # Any Go file
  - go.mod         # Dependencies
  - go.sum         # Dependency checksums
  - main.go        # Entry point
  - .github/workflows/pr-fast-feedback.yml  # Workflow itself
```

### Fast Feedback CI skips when:
```yaml
Only changed files are:
  - **.md          # Markdown docs
  - docs/**        # Documentation
  - scratchpad/**  # Scratch notes
  - LICENSE        # License file
```
