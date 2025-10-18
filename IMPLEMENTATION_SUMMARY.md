# COPY Command Implementation - Summary

## What Was Implemented

✅ **Complete COPY command support** for dockstep, matching Docker's standard COPY behavior.

## Key Features

### 1. Standard Docker COPY Syntax
Users can now use `COPY` commands directly in their block definitions:
```yaml
cmd: |
  COPY . /app
  COPY --chown=user:group src/ /app/src/
```

### 2. .dockerignore Support
- Automatically respects `.dockerignore` files in the build context
- Supports standard Docker ignore patterns (glob, negation, etc.)
- Filters files before creating tar archives

### 3. Flexible Build Context
- **Default**: Project root directory
- **Per-block override**: `context: "./custom-path"` in yaml
- **Global override**: `--context /path` CLI flag
- Priority: CLI flag > block context > project root

### 4. Dockerfile Export Compatibility
- COPY commands are preserved when exporting to Dockerfile
- Other commands become RUN directives
- Generated Dockerfiles are valid and can be used with `docker build`

## Files Created

### New Packages
1. **`buildcontext/buildcontext.go`**
   - Creates tar archives of build context
   - Recursively walks directories
   - Applies .dockerignore filters

2. **`buildcontext/dockerignore.go`**
   - Parses .dockerignore files
   - Implements Docker's ignore pattern matching

### Modified Files
1. **`types/types.go`**
   - Added `Context` field to Block struct
   - Added `CopyInstruction` struct

2. **`docker/client.go`**
   - Added `CopyToContainer()` method

3. **`engine/engine.go`**
   - Added `parseCopyCommands()` function
   - Added `executeCopyCommands()` method
   - Added `contextPath` field to Engine
   - Added `NewEngineWithContext()` constructor
   - Integrated COPY execution into block workflow

4. **`export/dockerfile.go`**
   - Modified to separate COPY from RUN commands
   - Preserves COPY commands in generated Dockerfiles

5. **`cmd/dockstep/main.go`**
   - Repurposed `--context` flag for build context directory
   - Wired up context path to Engine initialization

6. **`cmd/dockstep/ui_server.go`**
   - Added context field support in JSON APIs

7. **`README.md`**
   - Updated with COPY command documentation
   - Added examples and usage instructions

### Test Files
1. **`test-copy/`**
   - Complete test setup with .dockerignore
   - Sample files to copy
   - Build script
   - Files that should be ignored

2. **`test-copy-custom-context/`**
   - Tests custom context directory feature

### Documentation
1. **`COPY_IMPLEMENTATION.md`**
   - Detailed technical documentation
   - Architecture overview
   - Usage examples

2. **`test-copy/README.md`**
   - Test instructions
   - Expected behavior documentation

## How It Works

### Execution Flow

1. **Parse Block Command**
   - Extract COPY instructions from cmd field
   - Parse source, destination, and flags (--chown)

2. **Determine Context**
   - Use CLI --context flag, or
   - Use block's context field, or
   - Default to project root

3. **Create Build Context**
   - Load .dockerignore from context directory
   - Walk directory tree
   - Filter files based on .dockerignore
   - Create tar archive (streamed via pipe)

4. **Transfer to Container**
   - Create Docker container (without starting)
   - For each COPY instruction:
     - Create tar of source path
     - Use Docker API to copy into container
   - Start container
   - Execute remaining commands

5. **Export to Dockerfile**
   - COPY commands stay as COPY
   - Other commands become RUN
   - Generated Dockerfile is standard-compliant

## Usage Examples

### Simple Copy
```yaml
blocks:
  - id: "build"
    from: "node:18"
    cmd: |
      COPY package.json .
      COPY src/ ./src/
      npm install
```

### With .dockerignore
```
# .dockerignore
node_modules/
*.log
.git/
```

### Custom Context
```yaml
blocks:
  - id: "frontend"
    from: "node:18"
    context: "./frontend"
    cmd: |
      COPY . /app
```

### CLI Override
```bash
dockstep run build --context /custom/path
```

## Testing

Build the binary:
```bash
cd /Users/leonardmarcq/Desktop/dockstep
go build -o dockstep ./cmd/dockstep
```

Run tests:
```bash
cd test-copy
../dockstep run copy-files
```

## Benefits

1. **CI/CD Friendly**: Standard COPY syntax works in pipelines
2. **Docker Compatible**: Generated Dockerfiles use standard COPY
3. **Flexible**: Multiple ways to specify build context
4. **Efficient**: Respects .dockerignore to avoid copying unnecessary files
5. **Backward Compatible**: Existing mounts continue to work

## Implementation Quality

- ✅ No linter errors
- ✅ Compiles successfully
- ✅ Follows Go best practices
- ✅ Comprehensive error handling
- ✅ Well-documented
- ✅ Test cases included
- ✅ UI support included

