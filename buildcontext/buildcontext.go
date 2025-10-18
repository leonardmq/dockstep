package buildcontext

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CreateContextTar creates a tar archive of the specified context directory,
// respecting .dockerignore patterns if present
func CreateContextTar(contextDir string, dockerignorePath string) (io.Reader, error) {
	// Parse .dockerignore if it exists
	var ignorePatterns []string
	if dockerignorePath != "" {
		patterns, err := ParseDockerignore(dockerignorePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse .dockerignore: %w", err)
		}
		ignorePatterns = patterns
	}

	// Create a pipe to stream the tar data
	reader, writer := io.Pipe()

	go func() {
		defer writer.Close()

		tw := tar.NewWriter(writer)
		defer tw.Close()

		err := addToTar(tw, contextDir, "", ignorePatterns)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
	}()

	return reader, nil
}

// addToTar recursively adds files to the tar archive, respecting ignore patterns
func addToTar(tw *tar.Writer, basePath, relPath string, ignorePatterns []string) error {
	fullPath := filepath.Join(basePath, relPath)
	
	// Check if this path should be ignored
	if shouldIgnore(relPath, ignorePatterns) {
		return nil
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return err
	}

	// Create tar header
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}

	// Set the name in the archive (relative to context root)
	if relPath == "" {
		header.Name = "."
	} else {
		// For single files, use just the filename
		// For directories, preserve the relative path
		if info.IsDir() {
			header.Name = relPath
		} else {
			// Single file - use just the filename
			header.Name = filepath.Base(relPath)
		}
	}

	// Write header
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// If it's a directory, recurse into it
	if info.IsDir() {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			entryPath := filepath.Join(relPath, entry.Name())
			if err := addToTar(tw, basePath, entryPath, ignorePatterns); err != nil {
				return err
			}
		}
	} else {
		// For regular files, copy the content
		file, err := os.Open(fullPath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		if err != nil {
			return err
		}
	}

	return nil
}

// shouldIgnore checks if a path should be ignored based on .dockerignore patterns
func shouldIgnore(path string, patterns []string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}

		// Handle negation patterns (starting with !)
		negated := false
		if strings.HasPrefix(pattern, "!") {
			negated = true
			pattern = pattern[1:]
		}

		// Convert to glob pattern
		globPattern := convertToGlob(pattern)

		matched, err := filepath.Match(globPattern, path)
		if err != nil {
			continue
		}

		if matched {
			return !negated
		}
	}

	return false
}

// convertToGlob converts .dockerignore patterns to Go glob patterns
func convertToGlob(pattern string) string {
	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		pattern = pattern + "*"
	}

	// Handle patterns that should match anywhere in the path
	if !strings.HasPrefix(pattern, "/") && !strings.Contains(pattern, "/") {
		pattern = "**/" + pattern
	}

	return pattern
}
