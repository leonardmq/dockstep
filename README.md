# Dockstep

> **Jupyter Notebook but for Docker** - Build, debug, and iterate on Docker images interactively

Dockstep transforms Docker image building into an interactive, explorable experience. Think of it as Jupyter Notebooks, but for Dockerfiles - where each cell is a build step you can run, inspect, modify, and iterate on independently.

https://github.com/user-attachments/assets/20f2d9f0-55a2-4c9f-aace-b27ac7a09e7b

## Why Dockstep?

Building complex Docker images is often inefficient. You write a large Dockerfile, run `docker build`, wait for completion, encounter an error, and start over. Then you forgot which steps worked, which steps did not, which steps are cached and which one never ran. This workflow makes debugging and iteration time-consuming.

**Dockstep provides an interactive experience:**

- **Iterative Development**: Run individual steps, see results instantly, modify and re-run
- **Enhanced Debugging**: Inspect logs, diffs, and state at every step
- **Caching**: Dockstep uses the Docker SDK and builds one image per step run without rebuilding the steps it extends
- **Experimental Workflow**: Fork builds, try different approaches, rollback when needed
- **Visual Interface**: Web UI makes complex builds more manageable


## Getting Started

### One-Line Install (Recommended)
```bash
curl -sSL https://raw.githubusercontent.com/leonardmq/dockstep/main/scripts/install.sh | bash
```

### Create a Project

```bash
# Scaffold the project
mkdir myproject && cd myproject
dockstep init
```

### Dockstep UI

The Dockstep UI lets you interact with your build through a Notebook UI that lets you run different blocks independently, edit them and export each step to a Dockerfile. The changes you make in the Dockstep UI will be saved to your project file (`dockstep.yaml`).

To open the Dockstep Notebook in the browser:
```bash
# Make sure to initialize the project first
dockstep init
dockstep ui --open
```

The UI works seamlessly with the CLI - run `dockstep status`, `dockstep up`, or `dockstep export` and everything stays in sync.

### Build from Source
```bash
git clone https://github.com/leonardmq/dockstep.git
cd dockstep
make build
sudo mv bin/dockstep /$HOME/.local/bin/
```

## Updating

### Update to latest version
```bash
curl -sSL https://raw.githubusercontent.com/leonardmq/dockstep/main/scripts/update.sh | bash
```

### Check your version
```bash
dockstep --version
```

## Using the Dockstep UI

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
   - Add more blocks that extend previous steps

5. **Export when ready:**
```bash
dockstep export dockerfile my-block
```

## Real-World Examples

A Dockstep project's blocks is defined in the `dockstep.yaml` file at the root of your project. This block defines lines that you would otherwise write directly in a Dockerfile. 

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
dockstep init                    # Initialize new project and create dockstep.yaml
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

The `dockstep.yaml` file can be edited in the Dockstep UI in `Edit YAML`, or manually.

### Key Features

- **Native Dockerfile Instructions**: Use standard `RUN`, `COPY`, `ENV`, etc. Dockstep is a different UX on top of Docker, not a replacement.
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

## Contributing

We welcome contributions! Dockstep is open source and community-driven.

- **Found a bug?** [Open an issue](https://github.com/your-username/dockstep/issues)
- **Have an idea?** [Start a discussion](https://github.com/your-username/dockstep/discussions)
- **Want to code?** [Check out our contributing guide](CONTRIBUTING.md)

## License

Apache-2.0 license - see [LICENSE](LICENSE) for details.

---

<div align="center">

**Star Dockstep on GitHub if you find it useful!**

[![GitHub stars](https://img.shields.io/github/stars/leonardmq/dockstep?style=social)](https://github.com/leonardmq/dockstep)

</div>
