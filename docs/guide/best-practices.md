# Best Practices

This guide provides recommendations for optimal performance, reliability, and organization when using immich-go.

## Google Photos Migration

### Takeout Creation

#### ✅ Recommended Settings
- **Format**: Choose ZIP format for easier processing.
- **File Size**: Select maximum size (50GB) to minimize the number of parts.
- **Content**: Include all photos, videos, and metadata.
- **Download**: Ensure all parts are downloaded completely.

#### ⚠️ Common Pitfalls
- **Incomplete Downloads**: Verify all `takeout-001.zip`, `takeout-002.zip`, etc., files are present.
- **Mixed Formats**: Don't mix ZIP and TGZ formats in the same import.
- **Partial Takeouts**: Some Google takeouts may be incomplete—request a new one if many files are missing.

### Import Strategy

#### Large Collections (100k+ Photos)
```bash
# Conservative approach for maximum reliability
immich-go upload --google \
  --server=http://localhost:2283 \
  --api-key=your-api-key \
  --concurrent-tasks=4 \
  --client-timeout=60m \
  --pause-immich-jobs=true \
  --on-errors=continue \
  --session-tag \
  /path/to/takeout-*.zip
```

#### Medium Collections (10k-100k Photos)
```bash
# Balanced performance and reliability
immich-go upload --google \
  --server=http://localhost:2283 \
  --api-key=your-api-key \
  --concurrent-tasks=8 \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-burst=Stack \
  /path/to/takeout-*.zip
```

#### Small Collections (<10k Photos)
```bash
# Fast import with full processing
immich-go upload --google \
  --server=http://localhost:2283 \
  --api-key=your-api-key \
  --concurrent-tasks=12 \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-burst=Stack \
  --manage-heic-jpeg=StackCoverJPG \
  /path/to/takeout-*.zip
```

### Troubleshooting Import Issues

#### Many Files Not Imported
1. **Check Takeout Completeness**: Verify all parts are included.
   ```bash
   ls -la takeout-*.zip
   ```

2. **Force Import Unmatched Files**:
   ```bash
   immich-go upload --google \
     --include-unmatched \
     --server=... --api-key=... /path/to/takeout-*.zip
   ```

3. **Request New Takeout**: If data seems incomplete, create a new takeout for missing periods.

#### Resuming Interrupted Imports
- **Safe to Restart**: immich-go detects existing files and skips duplicates.
- **Use Session Tags**: Enable `--session-tag` to track what was imported when.
- **Check Logs**: Review captured output to identify where the import stopped.

## Performance Optimization

### Network Considerations

#### Gigabit LAN (Fast, Stable)
```bash
# High throughput configuration
--concurrent-tasks=16
--client-timeout=30m
--pause-immich-jobs=true
```

#### Internet Connection (Variable Speed)
```bash
# Adaptive configuration
--concurrent-tasks=4-8
--client-timeout=60m
--on-errors=continue
```

#### Slow/Unstable Network
```bash
# Conservative configuration
--concurrent-tasks=1-2
--client-timeout=120m
--on-errors=continue
```

### Server Considerations

#### Powerful Server (High CPU/RAM)
```bash
# Maximize server utilization
--concurrent-tasks=12-20
--pause-immich-jobs=false  # Let server handle both
--client-timeout=30m
```

#### Limited Server Resources
```bash
# Reduce server load
--concurrent-tasks=2-4
--pause-immich-jobs=true
--client-timeout=60m
```

#### NAS or Low-Power Server
```bash
# Minimal resource usage
--concurrent-tasks=1-2
--pause-immich-jobs=true
--client-timeout=180m
```

### Storage Considerations

#### Fast Storage (SSD)
- Use higher concurrency (8-16 uploads).
- Enable all file management features.
- Process larger batches.

#### Slow Storage (Traditional HDD)
- Reduce concurrency (2-4 uploads).
- Process smaller batches.
- Consider staging on faster temporary storage.

#### Network Storage
- Account for network latency in timeouts.
- Test with small batches first.
- Monitor network utilization.

## Organization Strategies

### Album Management

#### Folder-Based Albums
```bash
# Create albums from folder structure
immich-go upload \
  --folder-as-album=FOLDER \
  --album-path-joiner=" - " \
  --server=... --api-key=... /organized/photos/
```

**Benefits**: 
- Maintains existing organization.
- Easy to understand structure.
- Works well with hierarchical folder systems.

**Best For**: Already organized photo collections.

#### Date-Based Organization
```bash
# Let Immich organize by date, use tags for categories
immich-go upload \
  --tag="Source/Import2024" \
  --tag="Camera/Canon5D" \
  --session-tag \
  --server=... --api-key=... /photos/
```

**Benefits**:
- Chronological organization.
- Flexible tagging system.
- Better for mixed sources.

**Best For**: Large collections from multiple sources.

### Tagging Strategies

#### Hierarchical Tagging
```bash
# Geographic hierarchy
--tag="Location/Europe/France/Paris"

# Event hierarchy  
--tag="Events/2024/Wedding/Ceremony"

# Equipment hierarchy
--tag="Equipment/Camera/Canon/5D-Mark-IV"
```

#### Multi-Dimensional Tagging
```bash
# Combine multiple tag dimensions
--tag="Location/Paris" \
--tag="Event/Wedding" \
--tag="People/Family" \
--tag="Year/2024"
```

#### Source Tracking
```bash
# Always tag sources for future reference
--tag="Source/GooglePhotos" \
--tag="Import/$(date +%Y-%m-%d)" \
--session-tag
```

## Backup and Recovery

### Backup Strategies

#### 3-2-1 Backup Rule
1. **3 Copies**: Original + 2 backups.
2. **2 Different Media**: Local + cloud/external.
3. **1 Offsite**: Cloud or remote location.

#### Implementation Example
```bash
# Local backup (Copy 2)
immich-go archive --from-immich \
  --from-server=http://localhost:2283 \
  --from-api-key=your-api-key \
  --write-to-folder=/local-backup/immich

# Offsite backup (Copy 3) - sync local backup to cloud
rsync -av /local-backup/immich/ user@remote-server:/backups/immich/
```

#### Incremental Backups
```bash
#!/bin/bash
# Daily incremental backup script

YESTERDAY=$(date -d '1 day ago' '+%Y-%m-%d')
TODAY=$(date '+%Y-%m-%d')

immich-go archive --from-immich \
  --from-server=http://localhost:2283 \
  --from-api-key=your-api-key \
  --from-date-range="$YESTERDAY,$TODAY" \
  --write-to-folder="/backup/incremental/$TODAY"
```

### Testing and Validation

#### Pre-Migration Testing
```bash
# Test with small subset first
immich-go --log-level=DEBUG upload \
  --dry-run \
  --server=... --api-key=... /small-test-folder/
```

#### Backup Validation
```bash
# Test restore capability
immich-go upload \
  --server=http://test-server:2283 \
  --api-key=test-api-key \
  /backup/folder/2024/2024-01/
```

## Security and Privacy

### API Key Management

#### ✅ Best Practices
- **Separate Keys**: Use different API keys for different purposes.
- **Minimal Permissions**: Only grant necessary permissions.
- **Regular Rotation**: Rotate keys periodically.
- **Secure Storage**: Don't hardcode keys in scripts.

#### Script Security
```bash
#!/bin/bash
# Secure script example

# Read API key from secure file
API_KEY=$(cat ~/.config/immich-go/api-key)
chmod 600 ~/.config/immich-go/api-key  # Restrict permissions

# Or use environment variable
export IMMICH_GO_API_KEY="your-key"
immich-go upload --api-key="$IMMICH_GO_API_KEY" ...
```

### Network Security

#### SSL/TLS Configuration
```bash
# Always use HTTPS in production
immich-go upload \
  --server=https://immich.yourdomain.com \
  --api-key=your-key \
  /photos/

# Only use --skip-verify-ssl for testing/development
immich-go upload \
  --server=https://immich-dev.local \
  --skip-verify-ssl \
  --api-key=your-key \
  /photos/
```

## Migration Planning

### Pre-Migration Checklist

- [ ] Immich server properly configured and accessible.
- [ ] API key created with all necessary permissions.
- [ ] Sufficient storage space (estimate 1.5x source size).
- [ ] Network capacity planned (estimate transfer time).
- [ ] Backup of source data created.

### Post-Migration

#### Validation Steps
1. **Count Verification**: Compare source and destination counts.
2. **Spot Checks**: Verify random samples for quality.
3. **Metadata Check**: Ensure dates, locations, and albums are preserved.
4. **Organization Review**: Confirm albums and tags applied correctly.
