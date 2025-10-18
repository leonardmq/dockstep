package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"dockstep.dev/docker"
	"dockstep.dev/engine"
	"dockstep.dev/export"
	"dockstep.dev/store"
	"dockstep.dev/types"
)

// cmdInit creates skeleton dockstep.yaml and .dockstep/ structure
func cmdInit(args []string) error {
	// Check if dockstep.yaml already exists
	if _, err := os.Stat("dockstep.yaml"); err == nil {
		fmt.Println("dockstep.yaml already exists in current directory")
		fmt.Println("Use 'dockstep status' to see current project state")
		return nil
	}

	// Create .dockstep/ directory
	store := store.New(".")
	if err := store.Init(); err != nil {
		return fmt.Errorf("failed to initialize .dockstep directory: %w", err)
	}

	// Create skeleton dockstep.yaml
	configContent := `version: "1.0"
name: "my-project"

settings:
  network: "default"
  shell: "/bin/sh"

blocks:
  - id: "main"
    from: "alpine:latest"
    cmd: |
      echo "Hello from dockstep!"
      # Add your commands here
`

	if err := os.WriteFile("dockstep.yaml", []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create dockstep.yaml: %w", err)
	}

	fmt.Println("Initialized dockstep project")
	fmt.Println("Created dockstep.yaml with example configuration")
	fmt.Println("Created .dockstep/ directory structure")
	return nil
}

// cmdStatus shows ordered blocks with state
func cmdStatus(ctx context.Context, args []string, engine *engine.Engine, store *store.Store) error {
	// Load all block states
	states, err := store.GetBlockStates()
	if err != nil {
		return fmt.Errorf("failed to load block states: %w", err)
	}

	fmt.Println("Block Status:")
	fmt.Println("============")

	for _, block := range engine.GetProject().Blocks {
		state, exists := states[block.ID]
		if !exists {
			state = &types.BlockState{
				ID:     block.ID,
				Status: types.StatusPending,
			}
		}

		status := string(state.Status)
		if state.Digest != "" {
			status += fmt.Sprintf(" (digest: %s)", state.Digest[:12])
		}
		if state.Hash != "" {
			status += fmt.Sprintf(" (hash: %s)", state.Hash[:8])
		}

		fmt.Printf("  %s: %s\n", block.ID, status)
	}

	return nil
}

// cmdUp executes blocks in order
func cmdUp(ctx context.Context, args []string, engine *engine.Engine) error {
	upFlags := flag.NewFlagSet("up", flag.ExitOnError)
	force := upFlags.Bool("force", false, "Ignore cache for all blocks")
	from := upFlags.String("from", "", "Start from a specific block")
	continueOnError := upFlags.Bool("continue-on-error", false, "Continue despite failures")

	if err := upFlags.Parse(args); err != nil {
		return err
	}

	opts := types.UpOptions{
		Force:           *force,
		FromBlock:       *from,
		ContinueOnError: *continueOnError,
	}

	fmt.Println("Executing blocks...")
	if err := engine.RunUp(ctx, opts); err != nil {
		return err
	}

	fmt.Println("All blocks completed successfully")
	return nil
}

// cmdRun executes a single block
func cmdRun(ctx context.Context, args []string, engine *engine.Engine) error {
	if len(args) == 0 {
		return fmt.Errorf("block ID required")
	}

	blockID := args[0]

	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	noCache := runFlags.Bool("no-cache", false, "Force rerun")
	keepContainer := runFlags.Bool("keep-container", false, "Do not auto remove")

	if err := runFlags.Parse(args[1:]); err != nil {
		return err
	}

	opts := types.RunOptions{
		Force:         *noCache,
		KeepContainer: *keepContainer,
	}

	fmt.Printf("Executing block: %s\n", blockID)
	if err := engine.RunBlock(ctx, blockID, opts); err != nil {
		return err
	}

	fmt.Printf("Block %s completed successfully\n", blockID)
	return nil
}

// cmdLogs prints logs for a block
func cmdLogs(ctx context.Context, args []string, store *store.Store) error {
	if len(args) == 0 {
		return fmt.Errorf("block ID required")
	}

	blockID := args[0]

	logs, err := store.LoadLogs(blockID)
	if err != nil {
		return fmt.Errorf("failed to load logs for block %s: %w", blockID, err)
	}

	if len(logs) == 0 {
		fmt.Printf("No logs found for block %s\n", blockID)
		return nil
	}

	// Print logs
	io.Copy(os.Stdout, strings.NewReader(string(logs)))
	return nil
}

// cmdDiff shows filesystem changes for a block
func cmdDiff(ctx context.Context, args []string, engine *engine.Engine, store *store.Store) error {
	if len(args) == 0 {
		return fmt.Errorf("block ID required")
	}

	diffFlags := flag.NewFlagSet("diff", flag.ExitOnError)
	sinceRoot := diffFlags.Bool("since", false, "Show cumulative diff from root")
	jsonOut := diffFlags.Bool("json", false, "Output raw JSON")
	filter := diffFlags.String("filter", "", "Filter kinds: A|M|D (comma-separated)")

	if err := diffFlags.Parse(args[1:]); err != nil {
		return err
	}

	blockID := args[0]

	// Build chain up to block when --since is set
	blocks := engine.GetProject().Blocks
	var chain []types.Block
	if *sinceRoot {
		// collect from first up to blockID
		for _, b := range blocks {
			chain = append(chain, b)
			if b.ID == blockID {
				break
			}
		}
		if len(chain) == 0 || chain[len(chain)-1].ID != blockID {
			return fmt.Errorf("block %s not found", blockID)
		}
	}

	// Load diffs
	var diffs []types.DiffEntry
	if *sinceRoot {
		for _, b := range chain {
			d, err := store.LoadDiff(b.ID)
			if err != nil {
				// tolerate missing for earlier blocks
				continue
			}
			diffs = append(diffs, d...)
		}
	} else {
		d, err := store.LoadDiff(blockID)
		if err != nil {
			return fmt.Errorf("failed to load diff for block %s: %w", blockID, err)
		}
		diffs = d
	}

	// Filtering
	if *filter != "" {
		allowed := map[string]bool{}
		for _, k := range strings.Split(*filter, ",") {
			allowed[strings.TrimSpace(k)] = true
		}
		var out []types.DiffEntry
		for _, e := range diffs {
			if allowed[e.Kind] {
				out = append(out, e)
			}
		}
		diffs = out
	}

	if *jsonOut {
		// Raw JSON
		data, _ := json.MarshalIndent(diffs, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Human output
	for _, e := range diffs {
		fmt.Printf("%s %s\n", e.Kind, e.Path)
	}
	return nil
}

// cmdExport handles export commands
func cmdExport(ctx context.Context, args []string, engine *engine.Engine, store *store.Store, dockerClient *docker.Client) error {
	if len(args) == 0 {
		return fmt.Errorf("export type required (dockerfile or image)")
	}

	exportType := args[0]
	exportArgs := args[1:]

	switch exportType {
	case "dockerfile":
		return cmdExportDockerfile(ctx, exportArgs, engine)
	case "image":
		return cmdExportImage(ctx, exportArgs, engine, store, dockerClient)
	default:
		return fmt.Errorf("unknown export type: %s", exportType)
	}
}

// cmdExportDockerfile generates a Dockerfile
func cmdExportDockerfile(ctx context.Context, args []string, engine *engine.Engine) error {
	dockerfileFlags := flag.NewFlagSet("export dockerfile", flag.ExitOnError)
	output := dockerfileFlags.String("output", "", "Output file path")
	collapseRuns := dockerfileFlags.Bool("collapse-runs", false, "Collapse adjacent RUN commands")
	pinDigests := dockerfileFlags.Bool("pin-digests", false, "Pin base images to digests")

	if err := dockerfileFlags.Parse(args); err != nil {
		return err
	}

	if len(dockerfileFlags.Args()) == 0 {
		return fmt.Errorf("block ID required")
	}
	endBlockID := dockerfileFlags.Args()[0]

	opts := types.DockerfileOptions{
		Output:       *output,
		CollapseRuns: *collapseRuns,
		PinDigests:   *pinDigests,
	}

	dockerfile, err := export.GenerateDockerfile(engine.GetProject(), endBlockID, opts)
	if err != nil {
		return fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	if *output != "" {
		if err := os.WriteFile(*output, []byte(dockerfile), 0644); err != nil {
			return fmt.Errorf("failed to write Dockerfile: %w", err)
		}
		fmt.Printf("Dockerfile written to %s\n", *output)
	} else {
		fmt.Println(dockerfile)
	}

	return nil
}

// cmdExportImage tags and pushes an image
func cmdExportImage(ctx context.Context, args []string, engine *engine.Engine, store *store.Store, dockerClient *docker.Client) error {
	if len(args) == 0 {
		return fmt.Errorf("block ID required")
	}

	blockID := args[0]

	imageFlags := flag.NewFlagSet("export image", flag.ExitOnError)
	tag := imageFlags.String("tag", "", "Image tag (required)")
	push := imageFlags.Bool("push", false, "Push image to registry")

	if err := imageFlags.Parse(args[1:]); err != nil {
		return err
	}

	if *tag == "" {
		return fmt.Errorf("tag is required")
	}

	opts := types.ImageExportOptions{
		Tag:  *tag,
		Push: *push,
	}

	if err := export.TagImage(ctx, dockerClient, store, blockID, opts); err != nil {
		return fmt.Errorf("failed to export image: %w", err)
	}

	return nil
}
