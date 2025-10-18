# COPY Command Implementation

## Overview

This document describes the implementation of Docker `COPY` command support in dockstep.

## Implementation Summary

### 1. Core Components

#### buildcontext Package
- **Location**: `/buildcontext/`
- **Files**:
  - `buildcontext.go` - Creates tar archives of build context
  - `dockerignore.go` - Parses .dockerignore files
- **Features**:
  - Recursively archives directories
  - Respects .dockerignore patterns
  - Handles glob patterns and exclusions
  - Streams tar data via io.Pipe

#### Types Updates
- **Location**: `/types/types.go`
- **Changes**:
  - Added `Context string` field to `Block` struct
  - Added `CopyInstruction` struct for parsed COPY commands

#### Docker Client Extension
- **Location**: `/docker/client.go`
- **Changes**:
  - Added `CopyToContainer()` method to copy tar archives to containers

### 2. COPY Command Processing

#### Parsing (engine/engine.go)
- `parseCopyCommands()` - Extracts COPY commands from block cmd field
- Supports `--chown` flag
- Returns slice of `CopyInstruction` structs

#### Execution (engine/engine.go)
- `executeCopyCommands()` - Executes all COPY commands for a block
- Determines context directory (block.Context || engine.contextPath || projectRoot)
- Locates and parses .dockerignore
- Creates tar archive for each COPY source
- Transfers to container via Docker API

#### Integration
- COPY commands execute **after container creation** but **before container start**
- This ensures files are present when the container runs
- Failure in COPY commands causes container cleanup and error propagation

### 3. Configuration

#### Block-level Context
```yaml
blocks:
  - id: "my-block"
    context: "./custom-context"  # optional, relative or absolute
    cmd: |
      COPY . /app
```

#### Global Context Flag
```bash
dockstep run my-block --context /path/to/context
```

Priority: CLI flag > block.context > project root

### 4. Dockerfile Export

#### Location
`/export/dockerfile.go`

#### Changes
- Separates COPY and RUN commands when generating Dockerfiles
- COPY commands are preserved as-is
- Other commands become RUN directives
- Maintains proper order of operations

### 5. UI Support

#### Location
`/cmd/dockstep/ui_server.go`

#### Changes
- Added `context` field to block creation/update endpoints
- JSON serialization automatically includes context field

## Usage Examples

### Basic COPY
```yaml
blocks:
  - id: "copy-all"
    from: "alpine:latest"
    workdir: "/app"
    cmd: |
      COPY . /app
      ls -la /app
```

### COPY with .dockerignore
Create `.dockerignore` in project root:
```
node_modules/
*.log
.git/
```

### COPY with Custom Context
```yaml
blocks:
  - id: "custom-context"
    from: "node:18"
    context: "./frontend"
    cmd: |
      COPY package*.json ./
      COPY src/ ./src/
```

### COPY with --chown
```yaml
blocks:
  - id: "copy-owned"
    from: "alpine:latest"
    cmd: |
      COPY --chown=1000:1000 . /app
```

## Testing

Test directories created:
- `/test-copy/` - Basic COPY functionality with .dockerignore
- `/test-copy-custom-context/` - Tests custom context directory

Run tests:
```bash
cd test-copy
dockstep run copy-files
```

## Technical Details

### Build Context Tar Format
- Uses Go's `archive/tar` package
- Streams data via `io.Pipe` for memory efficiency
- Preserves file permissions and metadata
- Respects .dockerignore patterns using glob matching

### .dockerignore Processing
- Standard Docker .dockerignore syntax
- Supports glob patterns (*, **, ?, [])
- Supports negation patterns (!)
- Comments (#) and empty lines ignored

### Error Handling
- COPY errors prevent container start
- Container is cleaned up on COPY failure
- Detailed error messages include source and destination paths

## Compatibility

- Works with all Docker base images
- Compatible with existing mount functionality
- Respects Docker context best practices
- Exports to standard Dockerfile COPY syntax

## Future Enhancements

Possible improvements:
- Support for COPY --from=<stage> for multi-stage builds
- Caching of build context tars
- Progress reporting for large file copies
- Support for .dockerignore per-block paths

