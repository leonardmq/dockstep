package engine

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dockstep.dev/docker"
	"dockstep.dev/store"
	"dockstep.dev/types"
)

// Engine orchestrates block execution
type Engine struct {
	dockerClient *docker.Client
	store        *store.Store
	cache        *store.Cache
	project      *types.Project
	projectRoot  string
	contextPath  string
}

// NewEngine creates a new Engine instance
func NewEngine(dockerClient *docker.Client, store *store.Store, project *types.Project, projectRoot string) *Engine {
	cache := store.NewCache()
	return &Engine{
		dockerClient: dockerClient,
		store:        store,
		cache:        cache,
		project:      project,
		projectRoot:  projectRoot,
		contextPath:  projectRoot, // Default to project root
	}
}

// NewEngineWithContext creates a new Engine instance with a custom context path
func NewEngineWithContext(dockerClient *docker.Client, store *store.Store, project *types.Project, projectRoot, contextPath string) *Engine {
	cache := store.NewCache()
	return &Engine{
		dockerClient: dockerClient,
		store:        store,
		cache:        cache,
		project:      project,
		projectRoot:  projectRoot,
		contextPath:  contextPath,
	}
}

// GetProject returns the project configuration
func (e *Engine) GetProject() *types.Project {
	return e.project
}

// SetProject replaces the current project configuration
func (e *Engine) SetProject(project *types.Project) {
	e.project = project
}

// RunBlock executes a single block
func (e *Engine) RunBlock(ctx context.Context, blockID string, opts types.RunOptions) error {
	// Find the block
	var block types.Block
	found := false
	for _, b := range e.project.Blocks {
		if b.ID == blockID {
			block = b
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("block %s not found", blockID)
	}

	// Resolve parent image reference (for container create) and parent digest (for hashing)
	parentImageRef, parentDigest, err := e.resolveParent(ctx, block)
	if err != nil {
		return fmt.Errorf("failed to resolve parent digest: %w", err)
	}

	// Compute cache hash
	hash := store.ComputeBlockHash(block, parentDigest)

	// Check cache if not forced
	fmt.Printf("DEBUG: Force flag: %v, Hash: %s\n", opts.Force, hash)
	if !opts.Force {
		if cachedDigest, exists := e.cache.GetCachedDigest(hash); exists {
			fmt.Printf("DEBUG: Found cached digest: %s\n", cachedDigest)

			// Load existing logs from previous successful run
			// Try successful logs first, then fall back to regular logs
			var existingLogs []byte
			var err error

			if existingLogs, err = e.store.LoadSuccessfulLogs(blockID); err != nil || len(existingLogs) == 0 {
				// Fall back to regular logs if no successful logs found
				existingLogs, err = e.store.LoadLogs(blockID)
			}

			if err == nil && len(existingLogs) > 0 {
				fmt.Printf("DEBUG: Displaying cached logs for block %s\n", blockID)
				// Display the cached logs to stdout so they appear in the UI
				fmt.Print(string(existingLogs))
			} else {
				fmt.Printf("DEBUG: No existing logs found for cached block %s\n", blockID)
			}

			// Update block state as cached
			state := &types.BlockState{
				ID:        blockID,
				Status:    types.StatusCached,
				Digest:    cachedDigest,
				Hash:      hash,
				Timestamp: time.Now(),
			}
			if err := e.store.SaveBlockState(blockID, state); err != nil {
				return fmt.Errorf("failed to save cached state: %w", err)
			}
			return nil
		} else {
			fmt.Printf("DEBUG: No cached digest found\n")
		}
	} else {
		fmt.Printf("DEBUG: Force flag is true, skipping cache\n")
	}

	// Update state to running
	state := &types.BlockState{
		ID:        blockID,
		Status:    types.StatusRunning,
		Hash:      hash,
		Timestamp: time.Now(),
	}
	if err := e.store.SaveBlockState(blockID, state); err != nil {
		return fmt.Errorf("failed to save running state: %w", err)
	}

	// Clear logs before starting a fresh build (not cached)
	if err := e.store.ClearLogs(blockID); err != nil {
		fmt.Printf("Warning: failed to clear existing logs: %v\n", err)
	}

	// Build the block
	startTime := time.Now()
	digest, err := e.buildBlock(ctx, block, parentImageRef)
	duration := time.Since(startTime)
	exitCode := 0
	if err != nil {
		exitCode = 1
	}

	// Update state with results
	state.Duration = duration
	state.ExitCode = exitCode

	if err != nil {
		state.Status = types.StatusFailed
		state.Error = err.Error()
	} else if exitCode != 0 {
		state.Status = types.StatusFailed
		state.Error = fmt.Sprintf("command exited with code %d", exitCode)
	} else {
		state.Status = types.StatusSuccess
		state.Digest = digest

		// Save successful logs separately so they can be retrieved even after failed runs
		if successfulLogs, err := e.store.LoadLogs(blockID); err == nil && len(successfulLogs) > 0 {
			if err := e.store.SaveSuccessfulLogs(blockID, successfulLogs); err != nil {
				fmt.Printf("Warning: failed to save successful logs: %v\n", err)
			}
		}
	}

	// Save final state
	if err := e.store.SaveBlockState(blockID, state); err != nil {
		return fmt.Errorf("failed to save final state: %w", err)
	}

	// Update cache if successful
	if state.Status == types.StatusSuccess {
		if err := e.cache.SetCachedDigest(hash, digest); err != nil {
			return fmt.Errorf("failed to update cache: %w", err)
		}
	}

	return nil
}

// RunUp executes all blocks in order
func (e *Engine) RunUp(ctx context.Context, opts types.UpOptions) error {
	startIndex := 0

	// Find starting block if specified
	if opts.FromBlock != "" {
		for i, block := range e.project.Blocks {
			if block.ID == opts.FromBlock {
				startIndex = i
				break
			}
		}
	}

	// Execute blocks in order
	for i := startIndex; i < len(e.project.Blocks); i++ {
		block := e.project.Blocks[i]

		runOpts := types.RunOptions{
			Force: opts.Force,
		}

		if err := e.RunBlock(ctx, block.ID, runOpts); err != nil {
			if !opts.ContinueOnError {
				return fmt.Errorf("block %s failed: %w", block.ID, err)
			}
			// Continue on error, but log the failure
			fmt.Printf("Warning: block %s failed: %v\n", block.ID, err)
		}
	}

	return nil
}

// resolveParent resolves the parent image reference for container creation and the digest for hashing
func (e *Engine) resolveParent(ctx context.Context, block types.Block) (string, string, error) {
	return e.resolveParentWithVisited(ctx, block, make(map[string]bool))
}

// resolveParentWithVisited resolves parent with cycle detection
func (e *Engine) resolveParentWithVisited(ctx context.Context, block types.Block, visited map[string]bool) (string, string, error) {
	if block.From != "" {
		// Ensure image is available locally; use the reference for container create
		if err := e.dockerClient.PullImage(ctx, block.From); err != nil {
			return "", "", fmt.Errorf("failed to pull image %s: %w", block.From, err)
		}
		digest, err := e.dockerClient.InspectImage(ctx, block.From)
		if err != nil {
			return "", "", err
		}
		// Return the original image reference for FROM directive, digest for hashing
		return block.From, digest, nil
	}

	if block.FromBlock != "" {
		// Check for circular dependency
		if visited[block.ID] {
			return "", "", fmt.Errorf("circular dependency detected: block %s depends on itself", block.ID)
		}

		// Mark current block as visited
		visited[block.ID] = true
		defer delete(visited, block.ID)

		// If a specific version is requested, use that digest directly
		if block.FromBlockVersion != "" {
			// Find the parent block to get its original 'from' reference
			var parentBlock types.Block
			found := false
			for _, b := range e.project.Blocks {
				if b.ID == block.FromBlock {
					parentBlock = b
					found = true
					break
				}
			}
			if !found {
				return "", "", fmt.Errorf("parent block %s not found in project", block.FromBlock)
			}

			// If parent block has a 'from' field, use that as the image reference
			if parentBlock.From != "" {
				return parentBlock.From, block.FromBlockVersion, nil
			}

			// Otherwise use the digest directly
			return block.FromBlockVersion, block.FromBlockVersion, nil
		}

		// Check if parent block has been executed and has a digest
		state, err := e.store.LoadBlockState(block.FromBlock)
		if err != nil {
			// If state file doesn't exist, treat as not executed
			if os.IsNotExist(err) {
				state = &types.BlockState{ID: block.FromBlock, Digest: ""}
			} else {
				return "", "", fmt.Errorf("failed to load state for parent block %s: %w", block.FromBlock, err)
			}
		}

		// If parent block has no digest, it needs to be executed first
		if state.Digest == "" {
			// Find the parent block definition
			var parentBlock types.Block
			found := false
			for _, b := range e.project.Blocks {
				if b.ID == block.FromBlock {
					parentBlock = b
					found = true
					break
				}
			}
			if !found {
				return "", "", fmt.Errorf("parent block %s not found in project", block.FromBlock)
			}

			// Recursively resolve parent dependencies first
			_, _, err := e.resolveParentWithVisited(ctx, parentBlock, visited)
			if err != nil {
				return "", "", fmt.Errorf("failed to resolve parent dependencies for %s: %w", block.FromBlock, err)
			}

			// Execute the parent block
			runOpts := types.RunOptions{Force: false}
			if err := e.RunBlock(ctx, block.FromBlock, runOpts); err != nil {
				return "", "", fmt.Errorf("failed to execute parent block %s: %w", block.FromBlock, err)
			}

			// Reload the state after execution
			state, err = e.store.LoadBlockState(block.FromBlock)
			if err != nil {
				return "", "", fmt.Errorf("failed to reload state for parent block %s: %w", block.FromBlock, err)
			}
			if state.Digest == "" {
				return "", "", fmt.Errorf("parent block %s still has no digest after execution", block.FromBlock)
			}
		}

		// For from_block, we need to get the original image reference from the parent block
		// Find the parent block to get its original 'from' reference
		var parentBlock types.Block
		found := false
		for _, b := range e.project.Blocks {
			if b.ID == block.FromBlock {
				parentBlock = b
				found = true
				break
			}
		}
		if !found {
			return "", "", fmt.Errorf("parent block %s not found in project", block.FromBlock)
		}

		// If parent block has a 'from' field, use that as the image reference
		if parentBlock.From != "" {
			return parentBlock.From, state.Digest, nil
		}

		// If parent block also uses from_block, we need to resolve it recursively
		// For now, use the digest as fallback (this might need further refinement)
		return state.Digest, state.Digest, nil
	}

	return "", "", fmt.Errorf("block has no parent specified")
}

// sanitizeForDockerTag converts a block ID to a valid Docker image tag
func sanitizeForDockerTag(blockID string) string {
	// Docker image tags must be lowercase and can only contain [a-z0-9._-]
	// Replace spaces and other invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9._-]`)
	sanitized := reg.ReplaceAllString(strings.ToLower(blockID), "-")

	// Remove consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	sanitized = reg.ReplaceAllString(sanitized, "-")

	// Remove leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")

	// Ensure it's not empty and starts with a letter or number
	if sanitized == "" {
		sanitized = "block"
	}
	if !regexp.MustCompile(`^[a-z0-9]`).MatchString(sanitized) {
		sanitized = "block-" + sanitized
	}

	return sanitized
}

// buildBlock builds a single block and returns the resulting digest
func (e *Engine) buildBlock(ctx context.Context, block types.Block, parentImageRef string) (string, error) {
	// Generate Dockerfile content
	dockerfileContent := e.generateDockerfile(block, parentImageRef)

	// Determine context directory
	contextDir := e.contextPath
	if block.Context != "" {
		contextDir = block.Context
		// Make absolute if relative
		if !filepath.IsAbs(contextDir) {
			contextDir = filepath.Join(e.contextPath, contextDir)
		}
	}

	// Create temporary directory for build context
	tempDir, err := os.MkdirTemp("", "dockstep-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write Dockerfile to temp directory
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Copy build context files to temp directory
	if err := e.copyBuildContext(contextDir, tempDir); err != nil {
		return "", fmt.Errorf("failed to copy build context: %w", err)
	}

	// Generate tag for the image
	sanitizedID := sanitizeForDockerTag(block.ID)
	tag := fmt.Sprintf("dockstep-%s-%d", sanitizedID, time.Now().Unix())

	// Build the image using temp directory as context
	fmt.Printf("DEBUG: Building image with tag: %s\n", tag)
	fmt.Printf("DEBUG: Dockerfile content:\n%s\n", dockerfileContent)
	digest, err := e.buildImageWithLogs(ctx, tempDir, dockerfileContent, tag, block.ID)
	if err != nil {
		fmt.Printf("DEBUG: Build failed with error: %v\n", err)
		return "", fmt.Errorf("failed to build image: %w", err)
	}
	fmt.Printf("DEBUG: Build succeeded with digest: %s\n", digest)

	// Save image digest
	if err := e.store.SaveImageDigest(block.ID, digest); err != nil {
		fmt.Printf("Warning: failed to save image digest: %v\n", err)
	}

	// Save the actual Dockerfile content that was used to build this image
	if err := e.store.SaveDockerfileSnapshot(digest, dockerfileContent); err != nil {
		fmt.Printf("Warning: failed to save Dockerfile snapshot: %v\n", err)
	}

	// Append image history
	_ = e.store.SaveImageHistory(block.ID, types.ImageRecord{Tag: tag, Digest: digest, Timestamp: time.Now(), Dockerfile: dockerfileContent})

	return digest, nil
}

// generateDockerfile generates Dockerfile content for a block
func (e *Engine) generateDockerfile(block types.Block, parentImageRef string) string {
	var lines []string

	// Add FROM directive
	lines = append(lines, fmt.Sprintf("FROM %s", parentImageRef))
	lines = append(lines, "")

	// Add block instructions
	for _, instruction := range block.Instructions {
		lines = append(lines, instruction)
	}

	return strings.Join(lines, "\n")
}

// buildImageWithLogs builds an image and captures the build logs
func (e *Engine) buildImageWithLogs(ctx context.Context, contextDir, dockerfileContent, tag, blockID string) (string, error) {
	// Create log callback that appends to store
	logCallback := func(logChunk []byte) {
		if err := e.store.AppendLogs(blockID, logChunk); err != nil {
			fmt.Printf("Warning: failed to append build logs: %v\n", err)
		}
	}

	// Build with log streaming
	digest, err := e.dockerClient.BuildImageWithLogs(ctx, contextDir, dockerfileContent, tag, logCallback)
	if err != nil {
		// Even if build fails, we want to keep the logs for debugging
		return "", err
	}

	return digest, nil
}

// createContextTar creates a tar archive of the build context
func (e *Engine) createContextTar(contextDir string) (io.ReadCloser, error) {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer pipeWriter.Close()

		tarWriter := tar.NewWriter(pipeWriter)
		defer tarWriter.Close()

		err := filepath.Walk(contextDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip the root directory itself
			if path == contextDir {
				return nil
			}

			// Get relative path
			relPath, err := filepath.Rel(contextDir, path)
			if err != nil {
				return err
			}

			// Create tar header
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = relPath

			// Write header
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}

			// Write file content if it's a regular file
			if info.Mode().IsRegular() {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				_, err = io.Copy(tarWriter, file)
				if err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			pipeWriter.CloseWithError(err)
		}
	}()

	return pipeReader, nil
}

// copyBuildContext copies files from source to destination directory
func (e *Engine) copyBuildContext(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the source directory itself
		if path == srcDir {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip .dockstep directory
		if strings.HasPrefix(relPath, ".dockstep") {
			return nil
		}

		// Create destination path
		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			// Create directory
			return os.MkdirAll(dstPath, info.Mode())
		} else {
			// Copy file
			srcFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			dstFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			return err
		}
	})
}
