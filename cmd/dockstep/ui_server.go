package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	assets "dockstep.dev"
	"dockstep.dev/config"
	"dockstep.dev/docker"
	"dockstep.dev/engine"
	"dockstep.dev/export"
	"dockstep.dev/store"
	"dockstep.dev/types"
)

// Embedded UI disabled for now; serve placeholder until SPA exists

type uiServer struct {
	engine       *engine.Engine
	store        *store.Store
	dockerClient *docker.Client
	busyMu       sync.Mutex
	isBusy       bool
}

func (s *uiServer) setBusy(b bool) bool {
	s.busyMu.Lock()
	defer s.busyMu.Unlock()
	if b {
		if s.isBusy {
			return false
		}
		s.isBusy = true
		return true
	}
	s.isBusy = false
	return true
}

func (s *uiServer) routes() http.Handler {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("/api/project", s.handleProject)
	mux.HandleFunc("/api/block", s.handleBlock)
	mux.HandleFunc("/api/image", s.handleImage)
	mux.HandleFunc("/api/export/dockerfile", s.handleExportDockerfile)
	mux.HandleFunc("/api/dockerfile", s.handleDockerfileByDigest)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/up", s.handleUp)
	mux.HandleFunc("/api/run", s.handleRun)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/diff", s.handleDiff)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/lineage", s.handleLineage)

	// Static UI: serve built SPA when present; fallback to placeholder
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// 1) Allow override via environment variable (absolute path to dist)
		if uiDir := strings.TrimSpace(os.Getenv("DOCKSTEP_UI_DIR")); uiDir != "" {
			fs := http.Dir(uiDir)
			if f, err := fs.Open("index.html"); err == nil {
				_ = f.Close()
				http.FileServer(fs).ServeHTTP(w, r)
				return
			}
		}
		// 2) Try embedded assets (go:embed)
		if sub, err := fs.Sub(assets.UI, "ui/dist"); err == nil {
			if f, err := sub.Open("index.html"); err == nil {
				_ = f.Close()
				http.FileServer(http.FS(sub)).ServeHTTP(w, r)
				return
			}
		}
		// 3) Try ui/dist from project root
		root := s.store.RootPath()
		fs := http.Dir(root + "/ui/dist")
		if f, err := fs.Open("index.html"); err == nil {
			_ = f.Close()
			http.FileServer(fs).ServeHTTP(w, r)
			return
		}
		// Placeholder
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>dockstep UI</title></head><body><h1>dockstep UI</h1><p>SPA not built yet. In ui/: npm install && npm run build. Then re-run 'dockstep ui'.</p></body></html>"))
	})

	return mux
}

func (s *uiServer) handleProject(w http.ResponseWriter, r *http.Request) {
	proj := s.engine.GetProject()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(proj)
}

func (s *uiServer) handleBlock(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			ID               string `json:"id"`
			From             string `json:"from"`
			FromBlock        string `json:"from_block"`
			FromBlockVersion string `json:"from_block_version"`
			Workdir          string `json:"workdir"`
			Context          string `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		proj := s.engine.GetProject()
		// Basic defaults if not provided
		if body.ID == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		// Ensure unique id
		for _, b := range proj.Blocks {
			if b.ID == body.ID {
				http.Error(w, "duplicate id", http.StatusBadRequest)
				return
			}
		}
		nb := types.Block{ID: body.ID, From: body.From, FromBlock: body.FromBlock, FromBlockVersion: body.FromBlockVersion, Context: body.Context, Instructions: []string{"RUN echo 'new block'"}}
		proj.Blocks = append(proj.Blocks, nb)
		cfgPath := s.store.RootPath() + "/dockstep.yaml"
		if err := config.Validate(proj); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.Write(proj, cfgPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(nb)
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		for _, b := range s.engine.GetProject().Blocks {
			if b.ID == id {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(b)
				return
			}
		}
		http.NotFound(w, r)
	case http.MethodPut:
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		idv, ok := body["id"].(string)
		if !ok || idv == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		proj := s.engine.GetProject()
		// Update block fields
		found := false
		for i := range proj.Blocks {
			if proj.Blocks[i].ID == idv {
				if v, ok := body["instructions"].([]any); ok {
					var out []string
					for _, e := range v {
						if s, ok := e.(string); ok {
							out = append(out, s)
						}
					}
					proj.Blocks[i].Instructions = out
				}
				// Also handle 'cmd' field for backward compatibility
				if v, ok := body["cmd"].(string); ok && v != "" {
					proj.Blocks[i].Instructions = []string{v}
				}
				if v, ok := body["from"].(string); ok {
					proj.Blocks[i].From = v
					proj.Blocks[i].FromBlock = ""
				}
				if v, ok := body["from_block"].(string); ok {
					proj.Blocks[i].FromBlock = v
					if v != "" {
						proj.Blocks[i].From = ""
					}
				}
				if v, ok := body["from_block_version"].(string); ok {
					proj.Blocks[i].FromBlockVersion = v
				}
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}
		// Re-validate and persist
		// Find config file from store root
		cfgPath := s.store.RootPath() + "/dockstep.yaml"
		if err := config.Validate(proj); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.Write(proj, cfgPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(proj)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		proj := s.engine.GetProject()
		// Find and remove block
		found := false
		for i, b := range proj.Blocks {
			if b.ID == id {
				proj.Blocks = append(proj.Blocks[:i], proj.Blocks[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}
		// Allow empty project - no automatic block creation

		// Re-validate and persist
		cfgPath := s.store.RootPath() + "/dockstep.yaml"
		if err := config.Validate(proj); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.Write(proj, cfgPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Update engine with new project
		s.engine.SetProject(proj)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *uiServer) handleImage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		digest, err := s.store.LoadImageDigest(id)
		if err != nil || digest == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"digest": digest})
	case http.MethodDelete:
		digest := r.URL.Query().Get("digest")
		if digest == "" {
			http.Error(w, "digest required", http.StatusBadRequest)
			return
		}
		if err := s.dockerClient.DeleteImage(r.Context(), digest); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *uiServer) handleExportDockerfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		EndBlockID   string `json:"endBlockId"`
		CollapseRuns bool   `json:"collapseRuns"`
		PinDigests   bool   `json:"pinDigests"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	content, err := export.GenerateDockerfile(s.engine.GetProject(), body.EndBlockID, types.DockerfileOptions{CollapseRuns: body.CollapseRuns, PinDigests: body.PinDigests})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(content))
}

func (s *uiServer) handleDockerfileByDigest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	digest := r.URL.Query().Get("digest")
	if digest == "" {
		http.Error(w, "digest required", http.StatusBadRequest)
		return
	}
	content, err := s.store.LoadDockerfileSnapshot(digest)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(content))
}

func (s *uiServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	states, _ := s.store.GetBlockStates()
	type item struct {
		ID         string            `json:"id"`
		Status     types.BlockStatus `json:"status"`
		Digest     string            `json:"digest,omitempty"`
		Hash       string            `json:"hash,omitempty"`
		Timestamp  *time.Time        `json:"timestamp,omitempty"`
		DurationMs int64             `json:"durationMs,omitempty"`
		Error      string            `json:"error,omitempty"`
	}
	var out []item
	for _, b := range s.engine.GetProject().Blocks {
		st, ok := states[b.ID]
		it := item{ID: b.ID, Status: types.StatusPending}
		if ok {
			it.Status = st.Status
			it.Digest = st.Digest
			it.Hash = st.Hash
			t := st.Timestamp
			it.Timestamp = &t
			it.DurationMs = st.Duration.Milliseconds()
			it.Error = st.Error
		}
		out = append(out, it)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *uiServer) handleUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.setBusy(true) {
		http.Error(w, "Another run is in progress", http.StatusConflict)
		return
	}
	var req struct {
		Force           bool   `json:"force"`
		From            string `json:"from"`
		ContinueOnError bool   `json:"continueOnError"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	go func() {
		defer s.setBusy(false)
		_ = s.engine.RunUp(r.Context(), types.UpOptions{Force: req.Force, FromBlock: req.From, ContinueOnError: req.ContinueOnError})
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (s *uiServer) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.setBusy(true) {
		http.Error(w, "Another run is in progress", http.StatusConflict)
		return
	}
	var req struct {
		ID            string `json:"id"`
		Force         bool   `json:"force"`
		KeepContainer bool   `json:"keepContainer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		s.setBusy(false)
		return
	}
	// Run synchronously; keep connection open and return final result
	defer s.setBusy(false)
	err := s.engine.RunBlock(r.Context(), req.ID, types.RunOptions{Force: req.Force, KeepContainer: req.KeepContainer})
	st, _ := s.store.LoadBlockState(req.ID)
	w.Header().Set("Content-Type", "application/json")
	if err != nil || (st != nil && st.Status == types.StatusFailed) {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(st)
}

func (s *uiServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	follow := r.URL.Query().Get("follow") == "true"
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if !follow {
		// Try successful logs first, then fall back to regular logs
		var data []byte
		var err error

		if data, err = s.store.LoadSuccessfulLogs(id); err != nil || len(data) == 0 {
			// Fall back to regular logs if no successful logs found
			data, err = s.store.LoadLogs(id)
		}

		if err != nil {
			http.Error(w, "no logs", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
		return
	}
	// Basic SSE follow using polling for MVP
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	var lastLen int
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Try successful logs first, then fall back to regular logs
			var data []byte
			var err error

			if data, err = s.store.LoadSuccessfulLogs(id); err != nil || len(data) == 0 {
				// Fall back to regular logs if no successful logs found
				data, err = s.store.LoadLogs(id)
			}

			if err == nil {
				if len(data) > lastLen {
					chunk := data[lastLen:]
					_, _ = fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(string(chunk), "\n", "\ndata: "))
					flusher.Flush()
					lastLen = len(data)
				}
			}
		}
	}
}

func (s *uiServer) handleDiff(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	// Get the current image digest for this block
	digest, err := s.store.LoadImageDigest(id)
	if err != nil {
		// No image built yet, return empty diff
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]types.DiffEntry{})
		return
	}

	// Get the parent image digest
	parentDigest, err := s.getParentImageDigest(id)
	if err != nil {
		// No parent image, return empty diff
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]types.DiffEntry{})
		return
	}

	// Get diff between parent and current image
	diff, err := s.dockerClient.GetImageDiff(r.Context(), parentDigest, digest)
	if err != nil {
		// If diff fails, return empty array
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]types.DiffEntry{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diff)
}

// getParentImageDigest gets the parent image digest for a block
func (s *uiServer) getParentImageDigest(blockID string) (string, error) {
	// Load the project to find the block's parent
	configPath, err := config.FindConfigFile(s.store.RootPath())
	if err != nil {
		return "", err
	}

	project, err := config.Parse(configPath)
	if err != nil {
		return "", err
	}

	// Find the block
	var block *types.Block
	for _, b := range project.Blocks {
		if b.ID == blockID {
			block = &b
			break
		}
	}
	if block == nil {
		return "", fmt.Errorf("block %s not found", blockID)
	}

	// If block has a from_block, get that block's digest
	if block.FromBlock != "" {
		return s.store.LoadImageDigest(block.FromBlock)
	}

	// If block has a from image, we can't diff against it directly
	// Return empty string to indicate no parent image
	if block.From != "" {
		return "", fmt.Errorf("cannot diff against base image %s", block.From)
	}

	return "", fmt.Errorf("no parent found for block %s", blockID)
}

func (s *uiServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	cfgPath := s.store.RootPath() + "/dockstep.yaml"
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		_, _ = w.Write(data)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		// parse to ensure valid and to apply defaults
		var p types.Project
		if err := yaml.Unmarshal(body, &p); err != nil {
			http.Error(w, fmt.Sprintf("yaml error: %v", err), http.StatusBadRequest)
			return
		}
		if err := config.Validate(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(cfgPath, body, 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// reload engine project
		proj, err := config.Parse(cfgPath)
		if err == nil {
			s.engine.SetProject(proj)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *uiServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	recs, err := s.store.LoadImageHistory(id)
	if err != nil {
		http.Error(w, "no history", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(recs)
}

func (s *uiServer) handleLineage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	// Build lineage chain by walking up from_block references
	var lineage []map[string]interface{}
	visited := make(map[string]bool)

	var buildLineage func(blockID string) error
	buildLineage = func(blockID string) error {
		if visited[blockID] {
			return fmt.Errorf("circular dependency detected")
		}
		visited[blockID] = true

		// Find the block
		var block types.Block
		found := false
		for _, b := range s.engine.GetProject().Blocks {
			if b.ID == blockID {
				block = b
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("block %s not found", blockID)
		}

		// Get current state for this block
		state, _ := s.store.LoadBlockState(blockID)
		digest := ""
		timestamp := ""
		if state != nil {
			digest = state.Digest
			timestamp = state.Timestamp.Format("2006-01-02 15:04:05")
		}

		// Add to lineage
		lineage = append([]map[string]interface{}{{
			"id":                 block.ID,
			"from":               block.From,
			"from_block":         block.FromBlock,
			"from_block_version": block.FromBlockVersion,
			"digest":             digest,
			"timestamp":          timestamp,
		}}, lineage...)

		// If this block has a from_block, recurse
		if block.FromBlock != "" {
			return buildLineage(block.FromBlock)
		}

		return nil
	}

	if err := buildLineage(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lineage)
}

func cmdUI(args []string, eng *engine.Engine, st *store.Store, dockerClient *docker.Client) error {
	fs := flagSet("ui")
	port := fs.Int("port", 7689, "Port to serve UI")
	open := fs.Bool("open", true, "Open browser")
	if err := fs.Parse(args); err != nil {
		return err
	}

	srv := &uiServer{engine: eng, store: st, dockerClient: dockerClient}
	httpSrv := &http.Server{Addr: ":" + strconv.Itoa(*port), Handler: srv.routes()}

	go func() {
		if *open {
			time.Sleep(300 * time.Millisecond)
			_ = exec.Command("open", fmt.Sprintf("http://localhost:%d/", *port)).Start()
		}
	}()

	log.Printf("dockstep UI listening on http://localhost:%d\n", *port)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// minimal flagset to avoid global conflicts
func flagSet(name string) *simpleFlagSet {
	return &simpleFlagSet{name: name, values: map[string]string{}}
}

type simpleFlagSet struct {
	name   string
	values map[string]string
}

func (s *simpleFlagSet) Int(name string, value int, usage string) *int {
	v := value
	s.values[name] = strconv.Itoa(v)
	return &v
}
func (s *simpleFlagSet) Bool(name string, value bool, usage string) *bool {
	v := value
	if v {
		s.values[name] = "true"
	} else {
		s.values[name] = "false"
	}
	return &v
}
func (s *simpleFlagSet) Parse(args []string) error { return nil }
