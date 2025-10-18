package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"dockstep.dev/config"
	"dockstep.dev/docker"
	"dockstep.dev/engine"
	"dockstep.dev/store"
)

var (
	projectPath = flag.String("project", ".", "Project root directory")
	contextName = flag.String("context", "", "Docker context name")
	quiet       = flag.Bool("quiet", false, "Reduce output")
	noColor     = flag.Bool("no-color", false, "Disable ANSI colors")
	debug       = flag.Bool("debug", false, "Enable debug output")
)

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := flag.Args()[0]
	args := flag.Args()[1:]

	// Find project root
	projectRoot, err := filepath.Abs(*projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to resolve project path: %v\n", err)
		os.Exit(1)
	}

	// Handle init command without requiring config file
	if command == "init" {
		if err := cmdInit(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Find config file
	configPath, err := config.FindConfigFile(projectRoot)
	if err != nil {
		// Special handling for ui command
		if command == "ui" {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "Hint: Run 'dockstep init' first to create a dockstep.yaml file\n")
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	// Parse and validate config
	project, err := config.Parse(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse config: %v\n", err)
		os.Exit(2)
	}

	if err := config.Validate(project); err != nil {
		fmt.Fprintf(os.Stderr, "Error: config validation failed: %v\n", err)
		os.Exit(2)
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create Docker client: %v\n", err)
		os.Exit(3)
	}
	defer dockerClient.Close()

	// Create store
	store := store.New(projectRoot)
	if err := store.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize project store: %v\n", err)
		os.Exit(1)
	}

	// Create engine with context path
	var eng *engine.Engine
	if *contextName != "" {
		// Use -context flag as build context directory
		contextPath, err := filepath.Abs(*contextName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to resolve context path: %v\n", err)
			os.Exit(1)
		}
		eng = engine.NewEngineWithContext(dockerClient, store, project, projectRoot, contextPath)
	} else {
		eng = engine.NewEngine(dockerClient, store, project, projectRoot)
	}

	// Execute command
	ctx := context.Background()
	if err := executeCommand(ctx, command, args, eng, store, dockerClient); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func executeCommand(ctx context.Context, command string, args []string, engine *engine.Engine, store *store.Store, dockerClient *docker.Client) error {
	switch command {
	case "status":
		return cmdStatus(ctx, args, engine, store)
	case "up":
		return cmdUp(ctx, args, engine)
	case "run":
		return cmdRun(ctx, args, engine)
	case "logs":
		return cmdLogs(ctx, args, store)
	case "diff":
		return cmdDiff(ctx, args, engine, store)
	case "ui":
		return cmdUI(args, engine, store)
	case "export":
		return cmdExport(ctx, args, engine, store, dockerClient)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `dockstep - interactive, incremental Docker image builder

Usage:
  dockstep <command> [flags] [args]

Commands:
  init                    Create skeleton dockstep.yaml and .dockstep/
  status                  Show ordered blocks with state
  up                      Execute blocks in order
  run <id>                Execute a single block
  logs <id>               Print logs for a block
  diff <id>               Show filesystem changes for a block
  ui                      Launch local UI server
  export dockerfile <id>  Generate Dockerfile for a block and its ancestry
  export image <id>        Tag and push image

Global flags:
  -project <path>         Project root directory (default: .)
  -context <docker-context> Docker context name
  -quiet                  Reduce output
  -no-color               Disable ANSI colors
  -debug                  Enable debug output

`)
}
