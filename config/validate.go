package config

import (
	"fmt"

	"dockstep.dev/types"
)

// Validate validates a project configuration
func Validate(project *types.Project) error {
	if project.Version == "" {
		return fmt.Errorf("version is required")
	}

	if project.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Allow empty projects - users can start with no blocks

	// Check for duplicate block IDs (only if there are blocks)
	if len(project.Blocks) > 0 {
		blockIDs := make(map[string]bool)
		for _, block := range project.Blocks {
			if block.ID == "" {
				return fmt.Errorf("block ID cannot be empty")
			}

			if blockIDs[block.ID] {
				return fmt.Errorf("duplicate block ID: %s", block.ID)
			}
			blockIDs[block.ID] = true

			// Validate block
			if err := validateBlock(block, blockIDs); err != nil {
				return fmt.Errorf("block %s: %w", block.ID, err)
			}
		}

		// Check for circular dependencies
		if err := checkCircularDependencies(project.Blocks); err != nil {
			return err
		}
	}

	return nil
}

// validateBlock validates a single block
func validateBlock(block types.Block, allBlockIDs map[string]bool) error {
	// Check that either 'from' or 'from_block' is specified, but not both
	if block.From == "" && block.FromBlock == "" {
		return fmt.Errorf("either 'from' or 'from_block' must be specified")
	}

	if block.From != "" && block.FromBlock != "" {
		return fmt.Errorf("cannot specify both 'from' and 'from_block'")
	}

	// If from_block is specified, check that the referenced block exists
	if block.FromBlock != "" {
		if !allBlockIDs[block.FromBlock] {
			return fmt.Errorf("from_block '%s' does not exist", block.FromBlock)
		}
	}

	// Validate instructions array is not empty
	if len(block.Instructions) == 0 {
		return fmt.Errorf("instructions array cannot be empty")
	}

	return nil
}

// checkCircularDependencies checks for circular dependencies in block references
func checkCircularDependencies(blocks []types.Block) error {
	// Build dependency graph
	deps := make(map[string]string) // blockID -> parentBlockID
	for _, block := range blocks {
		if block.FromBlock != "" {
			deps[block.ID] = block.FromBlock
		}
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for blockID := range deps {
		if !visited[blockID] {
			if hasCycle(blockID, deps, visited, recStack) {
				return fmt.Errorf("circular dependency detected in block references")
			}
		}
	}

	return nil
}

// hasCycle performs DFS to detect cycles
func hasCycle(blockID string, deps map[string]string, visited, recStack map[string]bool) bool {
	visited[blockID] = true
	recStack[blockID] = true

	if parent, exists := deps[blockID]; exists {
		if !visited[parent] {
			if hasCycle(parent, deps, visited, recStack) {
				return true
			}
		} else if recStack[parent] {
			return true
		}
	}

	recStack[blockID] = false
	return false
}
