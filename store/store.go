package store

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dockstep.dev/types"
)

const (
	StateDir          = "state"
	LogsDir           = "logs"
	ImagesDir         = "images"
	CacheFile         = "cache/index.json"
	ArtifactsDir      = "artifacts"
	HistoryDir        = "history"
	DockerfilesSubDir = "dockerfiles"
)

// Store manages the .dockstep/ directory structure and state persistence
type Store struct {
	rootPath string
}

// New creates a new Store instance
func New(rootPath string) *Store {
	return &Store{rootPath: rootPath}
}

// RootPath returns the project root path associated with the store
func (s *Store) RootPath() string {
	return s.rootPath
}

// NewCache creates a new Cache instance
func (s *Store) NewCache() *Cache {
	return NewCache(s)
}

// Init creates the .dockstep/ directory structure
func (s *Store) Init() error {
	dirs := []string{
		filepath.Join(s.rootPath, ".dockstep"),
		filepath.Join(s.rootPath, ".dockstep", StateDir),
		filepath.Join(s.rootPath, ".dockstep", LogsDir),
		filepath.Join(s.rootPath, ".dockstep", ImagesDir),
		filepath.Join(s.rootPath, ".dockstep", ArtifactsDir),
		filepath.Join(s.rootPath, ".dockstep", HistoryDir),
		filepath.Join(s.rootPath, ".dockstep", HistoryDir, DockerfilesSubDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// SaveDockerfileSnapshot saves the Dockerfile content for a built image digest under history/dockerfiles/<digest>.Dockerfile
func (s *Store) SaveDockerfileSnapshot(digest string, content string) error {
	if digest == "" {
		return fmt.Errorf("empty digest for dockerfile snapshot")
	}
	path := filepath.Join(s.rootPath, ".dockstep", HistoryDir, DockerfilesSubDir, digest+".Dockerfile")
	return os.WriteFile(path, []byte(content), 0644)
}

// LoadDockerfileSnapshot loads the Dockerfile content for a built image digest
func (s *Store) LoadDockerfileSnapshot(digest string) (string, error) {
	if digest == "" {
		return "", fmt.Errorf("empty digest")
	}
	path := filepath.Join(s.rootPath, ".dockstep", HistoryDir, DockerfilesSubDir, digest+".Dockerfile")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveBlockState saves block state to state/<block-id>.json
func (s *Store) SaveBlockState(id string, state *types.BlockState) error {
	path := filepath.Join(s.rootPath, ".dockstep", StateDir, id+".json")
	return s.writeJSON(path, state)
}

// LoadBlockState loads block state from state/<block-id>.json
func (s *Store) LoadBlockState(id string) (*types.BlockState, error) {
	path := filepath.Join(s.rootPath, ".dockstep", StateDir, id+".json")
	var state types.BlockState
	if err := s.readJSON(path, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SaveLogs saves logs to logs/<block-id>.log
func (s *Store) SaveLogs(id string, logs []byte) error {
	path := filepath.Join(s.rootPath, ".dockstep", LogsDir, id+".log")
	return os.WriteFile(path, logs, 0644)
}

// AppendLogs appends logs to logs/<block-id>.log creating the file if needed
func (s *Store) AppendLogs(id string, logs []byte) error {
	path := filepath.Join(s.rootPath, ".dockstep", LogsDir, id+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(logs)
	return err
}

// ClearLogs truncates logs/<block-id>.log to zero length
func (s *Store) ClearLogs(id string) error {
	path := filepath.Join(s.rootPath, ".dockstep", LogsDir, id+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// LoadLogs loads logs from logs/<block-id>.log
func (s *Store) LoadLogs(id string) ([]byte, error) {
	path := filepath.Join(s.rootPath, ".dockstep", LogsDir, id+".log")
	return os.ReadFile(path)
}

// SaveSuccessfulLogs saves successful build logs to logs/<block-id>.success.log
func (s *Store) SaveSuccessfulLogs(id string, logs []byte) error {
	path := filepath.Join(s.rootPath, ".dockstep", LogsDir, id+".success.log")
	return os.WriteFile(path, logs, 0644)
}

// LoadSuccessfulLogs loads successful build logs from logs/<block-id>.success.log
func (s *Store) LoadSuccessfulLogs(id string) ([]byte, error) {
	path := filepath.Join(s.rootPath, ".dockstep", LogsDir, id+".success.log")
	return os.ReadFile(path)
}

// SaveImageDigest saves image digest to images/<block-id>.digest
func (s *Store) SaveImageDigest(id, digest string) error {
	path := filepath.Join(s.rootPath, ".dockstep", ImagesDir, id+".digest")
	return os.WriteFile(path, []byte(digest), 0644)
}

// LoadImageDigest loads image digest from images/<block-id>.digest
func (s *Store) LoadImageDigest(id string) (string, error) {
	path := filepath.Join(s.rootPath, ".dockstep", ImagesDir, id+".digest")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveImageHistory appends an image record to history/<block-id>.jsonl (newline-delimited JSON)
func (s *Store) SaveImageHistory(id string, rec types.ImageRecord) error {
	path := filepath.Join(s.rootPath, ".dockstep", HistoryDir, id+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(rec)
}

// LoadImageHistory loads image records for a block
func (s *Store) LoadImageHistory(id string) ([]types.ImageRecord, error) {
	path := filepath.Join(s.rootPath, ".dockstep", HistoryDir, id+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []types.ImageRecord
	dec := json.NewDecoder(f)
	for {
		var rec types.ImageRecord
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return out, nil
		}
		out = append(out, rec)
	}
	return out, nil
}

// writeJSON writes data as JSON to a file
func (s *Store) writeJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// readJSON reads JSON data from a file
func (s *Store) readJSON(path string, data interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(data)
}

// ComputeBlockHash computes a deterministic cache key for a block
func ComputeBlockHash(block types.Block, parentDigest string) string {
	// Create a deterministic hash based on block configuration and parent
	h := sha256.New()

	// Include parent digest
	h.Write([]byte(parentDigest))

	// Include block fields that affect execution
	h.Write([]byte(block.ID))
	h.Write([]byte(block.From))
	h.Write([]byte(block.FromBlock))
	h.Write([]byte(block.Context))

	// Include instructions
	for _, instruction := range block.Instructions {
		h.Write([]byte(instruction))
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// GetBlockStates loads all block states from the store
func (s *Store) GetBlockStates() (map[string]*types.BlockState, error) {
	states := make(map[string]*types.BlockState)

	stateDir := filepath.Join(s.rootPath, ".dockstep", StateDir)
	files, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return states, nil // No state files yet
		}
		return nil, err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			id := file.Name()[:len(file.Name())-5] // Remove .json extension
			state, err := s.LoadBlockState(id)
			if err != nil {
				return nil, fmt.Errorf("failed to load state for block %s: %w", id, err)
			}
			states[id] = state
		}
	}

	return states, nil
}

// Cleanup removes all state for a block and its descendants
func (s *Store) Cleanup(blockID string, descendants []string) error {
	// Remove state files
	allBlocks := append([]string{blockID}, descendants...)

	for _, id := range allBlocks {
		// Remove state file
		statePath := filepath.Join(s.rootPath, ".dockstep", StateDir, id+".json")
		os.Remove(statePath)

		// Remove logs
		logPath := filepath.Join(s.rootPath, ".dockstep", LogsDir, id+".log")
		os.Remove(logPath)

		// No diff files to remove in new schema

		// Remove image digest
		imagePath := filepath.Join(s.rootPath, ".dockstep", ImagesDir, id+".digest")
		os.Remove(imagePath)
	}

	return nil
}
