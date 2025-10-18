# Publishing Dockstep

This guide explains how to build the UI, compile the CLI, and publish downloadable binaries via GitHub Releases.

## Prerequisites
- Node.js 20+ and npm
- Go (uses version defined in `go.mod`)
- macOS/Linux shell to run the scripts

## Local Build

To build the UI and the CLI for your host platform:

```bash
make build
# binaries will be in ./bin/
```

To produce archives for multiple platforms:

```bash
make dist
# archives will be in ./bin/
```

## Versioning & Releases

We use semantic version tags like `v0.1.0`.

1. Ensure your `main` branch is green and contains the changes to release.
2. Update the changelog (optional) and README as needed.
3. Create a tag and push it:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Pushing a tag starting with `v` triggers the GitHub Actions workflow that:
- Builds the UI
- Cross-compiles the CLI for Linux/macOS/Windows (amd64 and arm64 where applicable)
- Archives the binaries (`.tar.gz` on Linux, `.zip` on macOS/Windows)
- Creates a GitHub Release and uploads all artifacts

## Manual Release (optional)

If you prefer a manual release:

1. Run `make dist` locally.
2. Create a new release on GitHub with tag `vX.Y.Z`.
3. Upload the generated archives from `bin/` to the release page.

## Troubleshooting

- If UI assets are missing, run `make ui`.
- If Go build fails on tag builds, ensure `ui/dist` is committed for the tagged ref or rely on the workflow step that builds it at CI time.
- For Windows builds, an `.exe` suffix is automatically added in CI archives.
