# Dockstep

Interactive, incremental Docker image builder.

Dockstep allows you to build Docker images step by step, with caching, rollback capabilities, and the ability to inspect diffs and logs at each step. You can extend blocks with other blocks and fork to produce different versions of a same base image.

Main use case is debugging complex images.

## Features

- **Incremental builds**: Build images step by step with automatic caching
- **Interactive workflow**: Run, inspect, and rollback individual steps
- **State management**: Persistent state with logs, diffs, and cache
- **Export capabilities**: Generate Dockerfiles or export final images
- **Dependency tracking**: Automatic parent resolution and circular dependency detection
- **Web UI (recommended)**: Visual editor for blocks; runs and edits `dockstep.yaml` behind the scenes

## Installation

```bash
go build -o dockstep ./cmd/dockstep
```

## Quick Start (UI first)

1. Initialize a new project:
```bash
dockstep init
```

2. Launch the UI (recommended):
```bash
dockstep ui --open
```
This opens your browser to the Dockstep UI. Use it to add/edit blocks, run steps, view logs and diffs. All changes are written back to `dockstep.yaml` automatically.

3. From the UI, click "Run All" to build. Alternatively, from the CLI:
```bash
dockstep up
```

4. Check status (from UI or CLI):
```bash
dockstep status
```

5. Export Dockerfile for a block (includes ancestry):
```bash
dockstep export dockerfile <block-id>
```

If you prefer editing the file directly, you can still modify `dockstep.yaml` by hand, but the UI is generally faster and less error‑prone.

## Using the UI (recommended)

The UI is the primary way to work with Dockstep. It provides a visual editor for blocks, a runner, and rich logs/diffs. Under the hood, it edits the same `dockstep.yaml` that the CLI uses.

Start the UI from your project root:
```bash
dockstep ui --open
```

Changes you make are saved to `dockstep.yaml`, so CLI commands like `dockstep up`, `dockstep status`, and `dockstep export` will reflect your UI edits.

## Dockstep CLI

While the CLI remains available, it should be viewed as secondary to the UI for day‑to‑day use.

### Core Commands

- `dockstep init` - Create skeleton dockstep.yaml and .dockstep/
- `dockstep status` - Show ordered blocks with state
- `dockstep up` - Execute blocks in order
- `dockstep run <id>` - Execute a single block
- `dockstep logs <id>` - Print logs for a block

### Export Commands

- `dockstep export dockerfile <id>` - Generate Dockerfile for a block and its ancestry
- `dockstep export image <id>` - Tag and push image

### Global Flags

- `--project <path>` - Project root directory (default: .)
- `--context <path>` - Build context directory for COPY commands (default: project root)
- `--quiet` - Reduce output
- `--no-color` - Disable ANSI colors
- `--debug` - Enable debug output

## Configuration

The `dockstep.yaml` file defines your build configuration:

```yaml
version: "1.0"
name: "my-project"

settings:
  network: "default"
  shell: "/bin/sh"

blocks:
  - id: "unique-block-id"
    from: "alpine:latest"          # or from_block: "parent-block-id"
    workdir: "/app"
    context: "."                   # optional: build context directory for COPY commands
    env:
      - "KEY=value"
    mounts:
      - source: "/host/path"
        target: "/container/path"
        mode: "ro"
    cmd: |
      COPY . /app                  # COPY commands are supported!
      echo "Your commands here"
    ephemeral: false
    resources:
      cpu: "1.0"
      memory: "512m"
    network: "default"
    export:
      labels:
        maintainer: "your-name"
      entrypoint: ["/bin/sh"]
      cmd: ["-c", "echo hello"]
```

## Examples

See the `examples/` directory for sample configurations:

- `basic.yaml` - Simple multi-step build
- `multi-stage.yaml` - Complex multi-stage build example

## COPY Command Support

Dockstep supports standard Docker `COPY` commands in block commands. The COPY command respects `.dockerignore` files in the build context.

```yaml
blocks:
  - id: "copy-files"
    from: "alpine:latest"
    workdir: "/app"
    context: "."                 # optional: override build context (default: project root)
    cmd: |
      COPY . /app
      COPY --chown=user:group src/ /app/src/
      echo "Files copied successfully"
```

**Build Context:**
- By default, the build context is the project root
- Use the `--context` flag to override: `dockstep run copy-files --context /path/to/context`
- Or set `context` field per block in your dockstep.yaml
- Place `.dockerignore` in the context directory to exclude files

**Alternative: Mounts**

You can also use `mounts` for development workflows where you want live file access:

```yaml
blocks:
  - id: "with-mounts"
    from: "alpine:latest"
    workdir: "/app"
    mounts:
      - source: "./src"
        target: "/host/src"
    cmd: |
      cp -r /host/src/* /app/
```

## State Management

Dockstep maintains state in the `.dockstep/` directory:

- `state/` - Block execution metadata
- `logs/` - stdout+stderr per block
- `diffs/` - filesystem changes per block
- `images/` - image digests
- `cache/` - cache index for incremental builds

## Development

This is an MVP implementation focusing on core functionality:

- ✅ Core flow (init, up, status, logs)
- ✅ Docker SDK integration
- ✅ JSON state persistence
- ✅ Basic CLI
- ✅ Export capabilities

## License

MIT
