package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dockstep.dev/buildcontext"
	"dockstep.dev/docker"
	"dockstep.dev/export"
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
	if !opts.Force {
		if cachedDigest, exists := e.cache.GetCachedDigest(hash); exists {
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
		}
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

	// Execute the block
	startTime := time.Now()
	digest, exitCode, err := e.executeBlock(ctx, block, parentImageRef)
	duration := time.Since(startTime)

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

		// For from_block, the stored digest is the image ID/reference; use it for both ref and hash parent
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

// executeBlock executes a single block and returns the resulting digest
func (e *Engine) executeBlock(ctx context.Context, block types.Block, parentImageRef string) (string, int, error) {
	// Parse COPY commands and remove them from the cmd
	copyInstructions := parseCopyCommands(block.Cmd)
	remainingCmd := removeCopyCommands(block.Cmd)
	
	fmt.Printf("DEBUG: Original cmd: %q\n", block.Cmd)
	fmt.Printf("DEBUG: Remaining cmd: %q\n", remainingCmd)
	
	// Create container configuration
	config := docker.ContainerConfig{
		Image:     parentImageRef,
		Cmd:       remainingCmd,
		Workdir:   block.Workdir,
		Env:       block.Env,
		Mounts:    e.convertMounts(block.Mounts),
		Network:   block.Network,
		Resources: block.Resources,
	}

	// Create container
	containerID, err := e.dockerClient.CreateContainer(ctx, config)
	if err != nil {
		return "", -1, fmt.Errorf("failed to create container: %w", err)
	}

	// Execute COPY commands before starting container
	if err := e.executeCopyCommands(ctx, block, containerID, copyInstructions); err != nil {
		e.dockerClient.RemoveContainer(ctx, containerID)
		return "", -1, fmt.Errorf("failed to execute COPY commands: %w", err)
	}

	// Start container
	fmt.Printf("DEBUG: Starting container %s\n", containerID)
	if err := e.dockerClient.StartContainer(ctx, containerID); err != nil {
		e.dockerClient.RemoveContainer(ctx, containerID)
		return "", -1, fmt.Errorf("failed to start container: %w", err)
	}
	fmt.Printf("DEBUG: Container started successfully\n")

	// Prepare logs: clear previous and start following logs, appending incrementally
	_ = e.store.ClearLogs(block.ID)
	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		_ = e.dockerClient.GetLogs(ctx, containerID, storeAppendWriter{store: e.store, id: block.ID}, true)
	}()

	// Wait for container to finish
	exitCode, err := e.dockerClient.WaitContainer(ctx, containerID)
	if err != nil {
		e.dockerClient.RemoveContainer(ctx, containerID)
		return "", -1, fmt.Errorf("failed to wait for container: %w", err)
	}

	// Give the logs follower a brief moment to drain remaining output
	select {
	case <-logsDone:
	case <-time.After(500 * time.Millisecond):
	}

	// Get filesystem diff
	diff, err := e.dockerClient.DiffContainer(ctx, containerID)
	if err != nil {
		fmt.Printf("Warning: failed to get diff: %v\n", err)
	} else {
		// Save diff
		if err := e.store.SaveDiff(block.ID, diff); err != nil {
			fmt.Printf("Warning: failed to save diff: %v\n", err)
		}
	}

	// Commit container if not ephemeral and successful
	var digest string
	if !block.Ephemeral && exitCode == 0 {
		sanitizedID := sanitizeForDockerTag(block.ID)
		tag := fmt.Sprintf("dockstep-%s-%d", sanitizedID, time.Now().Unix())
		digest, err = e.dockerClient.CommitContainer(ctx, containerID, tag)
		if err != nil {
			fmt.Printf("Warning: failed to commit container: %v\n", err)
		} else {
			// Save image digest
			if err := e.store.SaveImageDigest(block.ID, digest); err != nil {
				fmt.Printf("Warning: failed to save image digest: %v\n", err)
			}
			// Generate Dockerfile snapshot for this block at time of build and persist by digest
			if df, dfErr := export.GenerateDockerfile(e.project, block.ID, types.DockerfileOptions{}); dfErr != nil {
				fmt.Printf("Warning: failed to generate Dockerfile snapshot: %v\n", dfErr)
			} else {
				if err := e.store.SaveDockerfileSnapshot(digest, df); err != nil {
					fmt.Printf("Warning: failed to save Dockerfile snapshot: %v\n", err)
				}
			}
			// Append image history (without embedding dockerfile to keep JSONL light)
			_ = e.store.SaveImageHistory(block.ID, types.ImageRecord{Tag: tag, Digest: digest, Timestamp: time.Now()})
		}
	}

	// Remove container
	if err := e.dockerClient.RemoveContainer(ctx, containerID); err != nil {
		fmt.Printf("Warning: failed to remove container: %v\n", err)
	}

	return digest, exitCode, nil
}

// convertMounts converts dockstep mounts to Docker mount strings
func (e *Engine) convertMounts(mounts []types.Mount) []string {
	var dockerMounts []string
	for _, mount := range mounts {
		source := mount.Source
		// Resolve relative paths to absolute paths based on project root
		if !filepath.IsAbs(source) {
			source = filepath.Join(e.projectRoot, source)
		}
		mountStr := source + ":" + mount.Target
		if mount.Mode != "" {
			mountStr += ":" + mount.Mode
		}
		dockerMounts = append(dockerMounts, mountStr)
	}
	return dockerMounts
}

// parseCopyCommands parses COPY commands from a block's cmd field
func parseCopyCommands(cmd string) []types.CopyInstruction {
	var instructions []types.CopyInstruction

	lines := strings.Split(cmd, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(line), "COPY") {
			continue
		}

		// Parse COPY command: COPY [--chown=user:group] source dest
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		instruction := types.CopyInstruction{}
		i := 1 // Skip "COPY"

		// Check for --chown flag
		if i < len(parts) && strings.HasPrefix(parts[i], "--chown=") {
			instruction.Chown = strings.TrimPrefix(parts[i], "--chown=")
			i++
		}

		// Source and destination
		if i+1 < len(parts) {
			instruction.Source = parts[i]
			instruction.Dest = parts[i+1]
			instructions = append(instructions, instruction)
		}
	}

	return instructions
}

// removeCopyCommands removes COPY commands from a cmd string and returns the remaining commands
func removeCopyCommands(cmd string) string {
	lines := strings.Split(cmd, "\n")
	var remainingLines []string
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue // Skip empty lines
		}
		
		// Skip COPY commands
		if strings.HasPrefix(strings.ToUpper(trimmed), "COPY") {
			continue
		}
		
		remainingLines = append(remainingLines, line) // Keep original formatting
	}
	
	return strings.Join(remainingLines, "\n")
}

// executeCopyCommands executes COPY commands for a block
func (e *Engine) executeCopyCommands(ctx context.Context, block types.Block, containerID string, instructions []types.CopyInstruction) error {
	if len(instructions) == 0 {
		return nil
	}
	
	fmt.Printf("DEBUG: Found %d COPY instructions\n", len(instructions))

	// Determine context directory
	contextDir := e.contextPath
	if block.Context != "" {
		contextDir = block.Context
		// Make absolute if relative
		if !filepath.IsAbs(contextDir) {
			contextDir = filepath.Join(e.contextPath, contextDir)
		}
	}

	// Check for .dockerignore
	dockerignorePath := filepath.Join(contextDir, ".dockerignore")
	if _, err := os.Stat(dockerignorePath); os.IsNotExist(err) {
		dockerignorePath = ""
	}

	for _, instruction := range instructions {
		fmt.Printf("DEBUG: Processing COPY %s -> %s\n", instruction.Source, instruction.Dest)
		// Create tar for the source path
		sourcePath := filepath.Join(contextDir, instruction.Source)
		fmt.Printf("DEBUG: Source path: %s\n", sourcePath)
		tarReader, err := buildcontext.CreateContextTar(sourcePath, dockerignorePath)
		if err != nil {
			return fmt.Errorf("failed to create context tar for %s: %w", instruction.Source, err)
		}

		// Resolve destination path
		destPath := instruction.Dest
		if destPath == "." {
			// Use the block's working directory, or default to "/"
			if block.Workdir != "" {
				destPath = block.Workdir
			} else {
				destPath = "/"
			}
		} else if !strings.HasPrefix(destPath, "/") {
			// Make relative paths absolute
			if block.Workdir != "" {
				destPath = filepath.Join(block.Workdir, destPath)
			} else {
				destPath = "/" + destPath
			}
		}
		
		// For single file copies, ensure destination is a directory
		// Docker's CopyToContainer expects a directory path for extraction
		if !strings.HasSuffix(destPath, "/") {
			destPath = destPath + "/"
		}

		// Copy to container
		fmt.Printf("DEBUG: Copying to container %s at path %s\n", containerID, destPath)
		if err := e.dockerClient.CopyToContainer(ctx, containerID, destPath, tarReader); err != nil {
			return fmt.Errorf("failed to copy %s to %s: %w", instruction.Source, destPath, err)
		}
		fmt.Printf("DEBUG: Successfully copied %s to %s\n", instruction.Source, destPath)
	}

	return nil
}

// storeAppendWriter implements io.Writer to append logs to the store incrementally
type storeAppendWriter struct {
	store *store.Store
	id    string
}

func (w storeAppendWriter) Write(p []byte) (int, error) {
	if err := w.store.AppendLogs(w.id, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
