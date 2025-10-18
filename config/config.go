package config

import (
	"fmt"
	"os"
	"path/filepath"

	"dockstep.dev/types"
	"gopkg.in/yaml.v3"
)

// Parse loads and unmarshals a dockstep.yaml file
func Parse(path string) (*types.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var project types.Project
	if err := yaml.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply defaults
	applyDefaults(&project)

	return &project, nil
}

// applyDefaults sets default values for optional fields
func applyDefaults(project *types.Project) {
	// Set default network if not specified
	if project.Settings.Network == "" {
		project.Settings.Network = types.NetworkDefault
	}

	// Set default shell if not specified
	if project.Settings.Shell == "" {
		project.Settings.Shell = "/bin/sh"
	}

	// Apply settings defaults to blocks
	for i := range project.Blocks {
		block := &project.Blocks[i]

		// Apply network default
		if block.Network == "" {
			block.Network = project.Settings.Network
		}

		// Apply resources default
		if block.Resources == nil && project.Settings.Resources != nil {
			block.Resources = project.Settings.Resources
		}
	}
}

// FindConfigFile searches for dockstep.yaml in the given directory and parent directories
func FindConfigFile(startDir string) (string, error) {
	dir := startDir
	for {
		configPath := filepath.Join(dir, "dockstep.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}

	return "", fmt.Errorf("dockstep.yaml not found in %s or parent directories", startDir)
}
