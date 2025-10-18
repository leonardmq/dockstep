package config

import (
	"fmt"
	"os"

	"dockstep.dev/types"
	yaml "gopkg.in/yaml.v3"
)

// Write persists the project configuration back to a YAML file at the given path.
func Write(project *types.Project, path string) error {
	data, err := yaml.Marshal(project)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
