# Bolt's Journal

## 2025-02-18 - Optimization Strategy
**Learning:** `immich-go` spends significant time calculating checksums for files that are likely already on the server.
**Action:** Optimize `ShouldUpload` to check metadata (Name, Date, Size) *before* computing checksums. If a match is found and we are not overwriting, skip the expensive checksum calculation.

## 2025-02-18 - Code Clarity
**Learning:** Variable shadowing, or even the appearance of it (e.g., reusing variable names in adjacent blocks), can lead to confusion during code review and maintenance.
**Action:** Avoid reusing variable names like `ok` in complex logic flows, or rely on distinct properties (like `len(ids) > 0`) that don't depend on the shadowed variable.
