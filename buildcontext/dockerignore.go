package buildcontext

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseDockerignore parses a .dockerignore file and returns the ignore patterns
func ParseDockerignore(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .dockerignore: %w", err)
	}

	return patterns, nil
}
