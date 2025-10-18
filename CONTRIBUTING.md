# Contributing to Dockstep

Thank you for your interest in contributing to Dockstep! This project is licensed under the Apache 2.0 License.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/your-username/dockstep.git
   cd dockstep
   ```
3. **Build from source**:
   ```bash
   make build
   ```

## Development

### Project Structure
- `cmd/` - CLI commands and main entry point
- `engine/` - Core build engine
- `docker/` - Docker client integration
- `ui/` - Web UI (React/TypeScript)
- `types/` - Go type definitions
- `config/` - Configuration management
- `store/` - State and cache management

### Making Changes

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** and test them:
   ```bash
   # Test the CLI
   make test
   
   # Test the UI
   cd ui && npm test
   ```

3. **Commit your changes**:
   ```bash
   git commit -m "Add: brief description of your changes"
   ```

## Submitting Changes

1. **Push your branch**:
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Open a Pull Request** on GitHub with:
   - Clear description of what you changed
   - Reference any related issues
   - Screenshots for UI changes

## Code Style

- **Go**: Follow standard Go conventions
- **TypeScript/React**: Use Prettier and ESLint (configured in the project)
- **Commits**: Use conventional commit messages when possible

## License

By contributing to Dockstep, you agree that your contributions will be licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

## Questions?

- Open an [issue](https://github.com/leonardmq/dockstep/issues) for bugs or questions
- Start a [discussion](https://github.com/leonardmq/dockstep/discussions) for ideas or help
