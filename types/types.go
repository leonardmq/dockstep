package types

import (
	"time"
)

// BlockStatus represents the execution status of a block
type BlockStatus string

const (
	StatusPending BlockStatus = "pending"
	StatusCached  BlockStatus = "cached"
	StatusRunning BlockStatus = "running"
	StatusSuccess BlockStatus = "success"
	StatusFailed  BlockStatus = "failed"
	StatusSkipped BlockStatus = "skipped"
)

// NetworkMode represents the network configuration for a block
type NetworkMode string

const (
	NetworkDefault NetworkMode = "default"
	NetworkNone    NetworkMode = "none"
	NetworkHost    NetworkMode = "host"
)

// Mount represents a volume mount configuration
type Mount struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Mode   string `yaml:"mode,omitempty"`
}

// CopyInstruction represents a COPY command parsed from block cmd
type CopyInstruction struct {
	Source string
	Dest   string
	Chown  string
}

// Resources represents resource constraints for a block
type Resources struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// ExportConfig represents export settings for a block
type ExportConfig struct {
	Artifacts  []string          `yaml:"artifacts,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty"`
	Entrypoint []string          `yaml:"entrypoint,omitempty"`
	Cmd        []string          `yaml:"cmd,omitempty"`
}

// Block represents a single build step
type Block struct {
	ID        string        `yaml:"id"`
	From      string        `yaml:"from,omitempty"`
	FromBlock string        `yaml:"from_block,omitempty"`
	Workdir   string        `yaml:"workdir,omitempty"`
	Env       []string      `yaml:"env,omitempty"`
	Mounts    []Mount       `yaml:"mounts,omitempty"`
	Cmd       string        `yaml:"cmd"`
	Context   string        `yaml:"context,omitempty"`
	Ephemeral bool          `yaml:"ephemeral,omitempty"`
	Resources *Resources    `yaml:"resources,omitempty"`
	Network   NetworkMode   `yaml:"network,omitempty"`
	Export    *ExportConfig `yaml:"export,omitempty"`
}

// Settings represents default settings for the project
type Settings struct {
	Network   NetworkMode `yaml:"network,omitempty"`
	Shell     string      `yaml:"shell,omitempty"`
	Resources *Resources  `yaml:"resources,omitempty"`
}

// Project represents the complete dockstep configuration
type Project struct {
	Version  string   `yaml:"version"`
	Name     string   `yaml:"name"`
	Settings Settings `yaml:"settings,omitempty"`
	Blocks   []Block  `yaml:"blocks"`
}

// DiffEntry represents a filesystem change
type DiffEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // A=Added, M=Modified, D=Deleted
	Size int64  `json:"size,omitempty"`
}

// BlockState represents the execution state of a block
type BlockState struct {
	ID        string        `json:"id"`
	Status    BlockStatus   `json:"status"`
	Digest    string        `json:"digest,omitempty"`
	Hash      string        `json:"hash,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	ExitCode  int           `json:"exit_code,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// RunOptions represents options for running a single block
type RunOptions struct {
	Force         bool
	KeepContainer bool
}

// UpOptions represents options for running all blocks
type UpOptions struct {
	Force           bool
	FromBlock       string
	ContinueOnError bool
}

// DockerfileOptions represents options for Dockerfile export
type DockerfileOptions struct {
	Output       string
	CollapseRuns bool
	PinDigests   bool
}

// ImageExportOptions represents options for image export
type ImageExportOptions struct {
	Tag  string
	Push bool
}

// ImageRecord represents a single built image for a block
type ImageRecord struct {
	Tag        string    `json:"tag"`
	Digest     string    `json:"digest"`
	Timestamp  time.Time `json:"timestamp"`
	Dockerfile string    `json:"dockerfile,omitempty"`
}
