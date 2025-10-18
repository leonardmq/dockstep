# Dockstep

> **The Jupyter Notebook for Docker** - Build, debug, and iterate on Docker images interactively

Dockstep transforms Docker image building from a black-box process into an interactive, explorable experience. Think of it as Jupyter Notebooks, but for Dockerfiles - where each cell is a build step you can run, inspect, modify, and iterate on independently.

## Why Dockstep?

Building complex Docker images is often inefficient. You write a large Dockerfile, run `docker build`, wait for completion, encounter an error, and start over. This workflow makes debugging and iteration time-consuming.

**Dockstep provides a better approach:**

- **Iterative Development**: Run individual steps, see results instantly, modify and re-run
- **Enhanced Debugging**: Inspect logs, diffs, and state at every step
- **Intelligent Caching**: Only rebuild what changed, when it changed
- **Experimental Workflow**: Fork builds, try different approaches, rollback when needed
- **Visual Interface**: Web UI makes complex builds more manageable

## Use Cases

### **Debugging Complex Images**
```bash
# Instead of: docker build . (wait 15 minutes, get error, repeat)
dockstep run base-setup    # Works
dockstep run install-deps  # Fails - inspect logs, fix, re-run
dockstep run build-app     # Works
```

### **Rapid Prototyping**
Experiment with different base images, package managers, or build strategies without starting from scratch each time.

### **Learning Docker**
Understand what each Dockerfile instruction actually does by running them step-by-step and seeing the results.

### **Multi-Stage Builds**
Build complex multi-stage images with clear separation of concerns and easy debugging of each stage.

### **CI/CD Optimization**
Identify exactly which steps are slow or failing in your build pipeline, then optimize them individually.

### **Environment-Specific Builds**
Create different variants of the same base image (dev, staging, prod) by forking at specific steps.

## Features

- **Interactive Workflow**: Run, inspect, and rollback individual steps
- **Smart Caching**: Only rebuild what changed, when it changed
- **Rich Inspection**: View logs, diffs, and container state at every step
- **Web UI**: Visual editor that makes complex builds more intuitive
- **Export Ready**: Generate standard Dockerfiles or push images
- **Dependency Tracking**: Automatic parent resolution and circular dependency detection
- **State Management**: Persistent state with full build history

## Quick Start

### Option 1: Build from Source (Recommended)
```bash
git clone https://github.com/your-username/dockstep.git
cd dockstep
go build -o dockstep ./cmd/dockstep
sudo mv dockstep /usr/local/bin/
```

### Option 2: Download Binary
```bash
# Download latest release (coming soon)
curl -L https://github.com/your-username/dockstep/releases/latest/download/dockstep-darwin-amd64 -o dockstep
chmod +x dockstep
sudo mv dockstep /usr/local/bin/
```

## Get Started

1. **Initialize your project:**
```bash
dockstep init
```

2. **Launch the interactive UI:**
```bash
dockstep ui --open
```
This opens your browser to the Dockstep UI.

3. **Add your first build step:**
   - Click "Add Block" in the UI
   - Choose a base image (e.g., `node:18`, `python:3.9`, `alpine:latest`)
   - Add some commands (e.g., `RUN echo "Hello from Dockstep!"`)

4. **Run and iterate:**
   - Click "Run" on your block
   - See the results instantly
   - Modify and re-run as needed

5. **Export when ready:**
```bash
dockstep export dockerfile my-block
```

## The Dockstep UI

The UI provides an interactive Docker development environment. Think VS Code, but for Docker builds.

**What you get:**
- **Visual Block Editor**: Drag, drop, and configure build steps
- **Live Execution**: Run any step instantly and see results
- **Rich Logs & Diffs**: Deep dive into what each step actually does
- **Real-time State**: See your build state update as you work
- **Auto-sync**: All changes saved to `dockstep.yaml` automatically

**Start building:**
```bash
dockstep ui --open
```

The UI works seamlessly with the CLI - run `dockstep status`, `dockstep up`, or `dockstep export` and everything stays in sync.

## Real-World Examples

### Python Web App
```yaml
blocks:
  - id: "base"
    from: "python:3.9-slim"
    instructions:
      - "WORKDIR /app"
      - "RUN apt-get update && apt-get install -y gcc"
  
  - id: "deps"
    from_block: "base"
    instructions:
      - "COPY requirements.txt ."
      - "RUN pip install -r requirements.txt"
  
  - id: "app"
    from_block: "deps"
    instructions:
      - "COPY . ."
      - "EXPOSE 8000"
      - "CMD [\"python\", \"app.py\"]"
```

### Node.js Microservice
```yaml
blocks:
  - id: "node-base"
    from: "node:18-alpine"
    instructions:
      - "WORKDIR /app"
      - "RUN apk add --no-cache python3 make g++"
  
  - id: "install"
    from_block: "node-base"
    instructions:
      - "COPY package*.json ."
      - "RUN npm ci --only=production"
  
  - id: "build"
    from_block: "install"
    instructions:
      - "COPY . ."
      - "RUN npm run build"
      - "EXPOSE 3000"
      - "CMD [\"npm\", \"start\"]"
```

### Multi-Stage Production Build
```yaml
blocks:
  - id: "builder"
    from: "node:18"
    instructions:
      - "WORKDIR /app"
      - "COPY package*.json ."
      - "RUN npm ci"
      - "COPY . ."
      - "RUN npm run build"
  
  - id: "runtime"
    from: "nginx:alpine"
    instructions:
      - "COPY --from_block=builder /app/dist /usr/share/nginx/html"
      - "COPY nginx.conf /etc/nginx/nginx.conf"
      - "EXPOSE 80"
```

## CLI Reference

The CLI is designed for automation, CI/CD, and power users who prefer the command line.

### Core Commands
```bash
dockstep init                    # Initialize new project
dockstep status                  # Show build state
dockstep up                      # Build all blocks
dockstep run <block-id>          # Run specific block
dockstep logs <block-id>         # View block logs
```

### Export Commands
```bash
dockstep export dockerfile <id>  # Generate Dockerfile
dockstep export image <id>       # Tag and push image
```

### Global Flags
```bash
--project <path>    # Project root (default: .)
--context <path>    # Build context for COPY (default: project root)
--quiet            # Reduce output
--no-color         # Disable colors
--debug            # Enable debug output
```

## Dockstep vs Traditional Docker

| Traditional Docker | Dockstep |
|-------------------|----------|
| Write entire Dockerfile | Build step by step |
| Wait 15+ minutes for builds | Run individual steps in seconds |
| Cryptic error messages | Rich logs and diffs for each step |
| Start over on every failure | Fix and re-run just the broken step |
| No visibility into intermediate state | Inspect every layer and container |
| Hard to experiment | Fork, try, rollback easily |
| Text editor only | Visual interface |

## Perfect For

- **Developers** debugging complex builds
- **DevOps Engineers** optimizing CI/CD pipelines  
- **Students** learning Docker concepts
- **Researchers** experimenting with different configurations
- **Teams** standardizing build processes
- **Startups** rapidly prototyping containerized apps

## Configuration

Dockstep uses a simple YAML configuration file that's easy to read and version control:

```yaml
version: "1.0"
name: "my-awesome-app"

blocks:
  - id: "base"
    from: "node:18-alpine"
    instructions:
      - "WORKDIR /app"
      - "RUN apk add --no-cache python3 make g++"
  
  - id: "dependencies"
    from_block: "base"
    instructions:
      - "COPY package*.json ."
      - "RUN npm ci --only=production"
  
  - id: "app"
    from_block: "dependencies"
    context: "./src"              # Custom build context
    instructions:
      - "COPY . ."
      - "RUN npm run build"
      - "EXPOSE 3000"
      - "CMD [\"npm\", \"start\"]"
    export:
      labels:
        maintainer: "your-name@company.com"
        version: "1.0.0"
```

### Key Features

- **Native Dockerfile Instructions**: Use standard `RUN`, `COPY`, `ENV`, etc.
- **Block Dependencies**: Chain blocks with `from_block` references
- **Flexible Context**: Override build context per block or globally
- **Rich Metadata**: Add labels, entrypoints, and commands
- **.dockerignore Support**: Automatic file filtering

## Project Structure

```
my-project/
├── dockstep.yaml          # Your build configuration
├── .dockstep/             # Dockstep state (auto-generated)
│   ├── state/             # Execution metadata
│   ├── logs/              # Build logs per block
│   ├── images/            # Image digests
│   └── cache/             # Incremental build cache
├── .dockerignore          # Files to exclude from builds
└── src/                   # Your application code
```

## What's Next?

Dockstep is actively developed with features coming soon:

- **Real-time Collaboration**: Share builds with your team
- **Build Analytics**: Performance insights and optimization suggestions
- **Template Gallery**: Pre-built blocks for common patterns
- **Plugin System**: Extend Dockstep with custom functionality
- **Cloud Integration**: Deploy directly to your favorite cloud provider

## Contributing

We welcome contributions! Dockstep is open source and community-driven.

- **Found a bug?** [Open an issue](https://github.com/your-username/dockstep/issues)
- **Have an idea?** [Start a discussion](https://github.com/your-username/dockstep/discussions)
- **Want to code?** [Check out our contributing guide](CONTRIBUTING.md)

## License

MIT License - see [LICENSE](LICENSE) for details.

---

<div align="center">

**Star Dockstep on GitHub if you find it useful!**

[![GitHub stars](https://img.shields.io/github/stars/your-username/dockstep?style=social)](https://github.com/your-username/dockstep)
[![Twitter Follow](https://img.shields.io/twitter/follow/dockstep?style=social)](https://twitter.com/dockstep)

*Made with ❤️ by developers, for developers*

</div>
