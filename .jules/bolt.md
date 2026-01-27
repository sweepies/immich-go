## 2026-01-27 - [Duplicate File Upload Optimization]
**Learning:** Checking for file existence on the server by Name, Size, and Date BEFORE computing the file checksum (SHA1) avoids reading the entire file content for already-uploaded files.
**Action:** When implementing deduplication or incremental sync logic, always prioritize metadata checks (name, size, mtime) over content hashing if the risk of collision is acceptable or if the server supports it. This saves significant I/O and CPU time for large libraries.
