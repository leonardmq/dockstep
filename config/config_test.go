package config

import (
	"os"
	"path/filepath"
	"testing"

	"dockstep.dev/types"
)

func TestParse(t *testing.T) {
	// Create a temporary config file
	configContent := `version: "1.0"
name: "test-project"

settings:
  network: "default"
  shell: "/bin/sh"

blocks:
  - id: "base"
    from: "alpine:latest"
    cmd: "echo hello"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dockstep.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	project, err := Parse(configPath)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if project.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", project.Version)
	}

	if project.Name != "test-project" {
		t.Errorf("Expected name test-project, got %s", project.Name)
	}

	if len(project.Blocks) != 1 {
		t.Errorf("Expected 1 block, got %d", len(project.Blocks))
	}

	block := project.Blocks[0]
	if block.ID != "base" {
		t.Errorf("Expected block ID base, got %s", block.ID)
	}

	if block.From != "alpine:latest" {
		t.Errorf("Expected from alpine:latest, got %s", block.From)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		project *types.Project
		wantErr bool
	}{
		{
			name: "valid project",
			project: &types.Project{
				Version: "1.0",
				Name:    "test",
				Blocks: []types.Block{
					{
						ID:   "base",
						From: "alpine:latest",
						Cmd:  "echo hello",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing version",
			project: &types.Project{
				Name: "test",
				Blocks: []types.Block{
					{
						ID:   "base",
						From: "alpine:latest",
						Cmd:  "echo hello",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate block ID",
			project: &types.Project{
				Version: "1.0",
				Name:    "test",
				Blocks: []types.Block{
					{
						ID:   "base",
						From: "alpine:latest",
						Cmd:  "echo hello",
					},
					{
						ID:   "base",
						From: "alpine:latest",
						Cmd:  "echo world",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "circular dependency",
			project: &types.Project{
				Version: "1.0",
				Name:    "test",
				Blocks: []types.Block{
					{
						ID:        "a",
						FromBlock: "b",
						Cmd:       "echo a",
					},
					{
						ID:        "b",
						FromBlock: "a",
						Cmd:       "echo b",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.project)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test file not found
	_, err := FindConfigFile(tmpDir)
	if err == nil {
		t.Error("Expected error when config file not found")
	}

	// Create config file
	configPath := filepath.Join(tmpDir, "dockstep.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1.0"), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test file found
	found, err := FindConfigFile(tmpDir)
	if err != nil {
		t.Fatalf("Failed to find config file: %v", err)
	}

	if found != configPath {
		t.Errorf("Expected %s, got %s", configPath, found)
	}
}
