# Troubleshooting

This guide covers common issues and how to resolve them when using `immich-go`.

## Common Issues

### Upload Failures

#### Network Timeouts
If you encounter timeout errors during large video uploads, increase the client timeout:
```bash
--client-timeout=60m
```

#### Server Overload
If the Immich server becomes unresponsive, reduce the number of concurrent tasks:
```bash
--concurrent-tasks=2
```

### Google Photos Takeout Issues

#### Missing Metadata
Google Photos Takeout is notoriously complex. If files are being skipped due to missing metadata:
1. Verify all `takeout-*.zip` parts are in the same directory.
2. Use the `--include-unmatched` flag to import files even if their JSON metadata is missing.

#### Duplicate Detection
`immich-go` uses checksums to detect duplicates. If it reports a file "already exists," it means the exact same file is already on your Immich server. This is normal when resuming an interrupted upload.

## How to Share Data with Developers

If you encounter a bug or a specific Takeout structure that `immich-go` doesn't handle correctly, sharing a file list can help us troubleshoot without you needing to send your actual photos.

### Generate File List (Linux / macOS / WSL)

```bash
for f in *.zip; do echo "Part: $f"; unzip -l "$f"; done > list.lst
```

### Generate File List (Windows PowerShell)

Use the following command in the folder containing your ZIP files:

```powershell
Get-ChildItem *.zip | ForEach-Object {
    Write-Output "Part: $($_.Name)"
    Expand-Archive -Path $_.FullName -ShowProxy | Out-Null # This is not efficient for listing
    # Better to use a dedicated tool or the ZipContents.ps1 script mentioned in the repository
}
```

::: tip Privacy
The file list only reveals filenames, sizes, and album names. It does not contain the actual image data.
:::

## Debugging

To get more detailed information about what `immich-go` is doing, increase the log level:

```bash
immich-go --log-level=DEBUG upload ...
```

To see the raw API requests and responses:

```bash
immich-go --api-trace upload ...
```
