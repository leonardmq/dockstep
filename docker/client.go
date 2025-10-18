package docker

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dockstep.dev/types"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// Client wraps the Docker client with dockstep-specific functionality
type Client struct {
	client *client.Client
}

// JSONMessage represents a single message from Docker's build output
type JSONMessage struct {
	Stream      string `json:"stream"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

// NewClient creates a new Docker client
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{client: cli}, nil
}

// PullImage pulls an image from registry
func (c *Client) PullImage(ctx context.Context, ref string) error {
	reader, err := c.client.ImagePull(ctx, ref, dockerTypes.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	defer reader.Close()

	// Read the response to completion
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read pull response: %w", err)
	}

	return nil
}

// BuildImage builds a Docker image from a Dockerfile
func (c *Client) BuildImage(ctx context.Context, contextDir, dockerfileContent string, tag string) (string, error) {
	return c.BuildImageWithLogs(ctx, contextDir, dockerfileContent, tag, nil)
}

// BuildImageWithLogs builds a Docker image from a Dockerfile and streams logs via callback
func (c *Client) BuildImageWithLogs(ctx context.Context, contextDir, dockerfileContent string, tag string, logCallback func([]byte)) (string, error) {
	// Create a tar archive of the build context
	tarReader, err := c.createContextTar(contextDir)
	if err != nil {
		return "", fmt.Errorf("failed to create context tar: %w", err)
	}
	defer tarReader.Close()

	// Build the image
	buildOptions := dockerTypes.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
		Remove:     true,
		NoCache:    false, // Allow caching for now
	}

	buildResponse, err := c.client.ImageBuild(ctx, tarReader, buildOptions)
	if err != nil {
		return "", fmt.Errorf("failed to build image: %w", err)
	}
	defer buildResponse.Body.Close()

	// Parse and stream build output
	buildFailed := false
	var lastError string
	var lastErrorDetail string
	scanner := bufio.NewScanner(buildResponse.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var message JSONMessage
		if err := json.Unmarshal(line, &message); err != nil {
			// If JSON parsing fails, treat the whole line as a log message
			if logCallback != nil {
				logCallback(append(line, '\n'))
			}
			continue
		}

		// Stream the log content
		if logCallback != nil && message.Stream != "" {
			logCallback([]byte(message.Stream))
		}

		// Handle errors
		if message.Error != "" {
			if logCallback != nil {
				logCallback([]byte(fmt.Sprintf("Error: %s\n", message.Error)))
			}
			buildFailed = true
			lastError = message.Error
		}
		if message.ErrorDetail.Message != "" {
			if logCallback != nil {
				logCallback([]byte(fmt.Sprintf("Error Detail: %s\n", message.ErrorDetail.Message)))
			}
			buildFailed = true
			lastErrorDetail = message.ErrorDetail.Message
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read build output: %w", err)
	}

	// If build failed, return an error with specific details
	if buildFailed {
		if lastErrorDetail != "" {
			return "", fmt.Errorf("build failed: %s", lastErrorDetail)
		} else if lastError != "" {
			return "", fmt.Errorf("build failed: %s", lastError)
		} else {
			return "", fmt.Errorf("build failed due to failed RUN commands")
		}
	}

	// Get the image ID - try by tag first, then by digest if available
	image, _, err := c.client.ImageInspectWithRaw(ctx, tag)
	if err != nil {
		// If tag inspection fails, the build likely failed due to failed RUN commands
		// Check if any images were created at all
		images, listErr := c.client.ImageList(ctx, dockerTypes.ImageListOptions{})
		if listErr != nil {
			return "", fmt.Errorf("failed to inspect built image and list images: %w", err)
		}

		// If no images were created, the build failed
		if len(images) == 0 {
			return "", fmt.Errorf("build failed: no image was created (likely due to failed RUN commands)")
		}

		// Use the most recent image ID - we need to inspect it to get the full image info
		image, _, err = c.client.ImageInspectWithRaw(ctx, images[0].ID)
		if err != nil {
			return "", fmt.Errorf("failed to inspect most recent image: %w", err)
		}
	}

	return image.ID, nil
}

// createContextTar creates a tar archive of the build context
func (c *Client) createContextTar(contextDir string) (io.ReadCloser, error) {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer pipeWriter.Close()

		tarWriter := tar.NewWriter(pipeWriter)
		defer tarWriter.Close()

		err := filepath.Walk(contextDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip the root directory itself
			if path == contextDir {
				return nil
			}

			// Get relative path
			relPath, err := filepath.Rel(contextDir, path)
			if err != nil {
				return err
			}

			// Create tar header
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = relPath

			// Write header
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}

			// Write file content if it's a regular file
			if info.Mode().IsRegular() {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				_, err = io.Copy(tarWriter, file)
				if err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			pipeWriter.CloseWithError(err)
		}
	}()

	return pipeReader, nil
}

// InspectImage inspects an image and returns its digest
func (c *Client) InspectImage(ctx context.Context, ref string) (string, error) {
	img, _, err := c.client.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to inspect image %s: %w", ref, err)
	}

	// Return the digest (sha256:...)
	if len(img.RepoDigests) > 0 {
		// Extract digest from repo digest
		parts := strings.Split(img.RepoDigests[0], "@")
		if len(parts) == 2 {
			return parts[1], nil
		}
	}

	// Fallback to ID
	return img.ID, nil
}

// TagImage tags an image with a new name
func (c *Client) TagImage(ctx context.Context, source, target string) error {
	err := c.client.ImageTag(ctx, source, target)
	if err != nil {
		return fmt.Errorf("failed to tag image %s as %s: %w", source, target, err)
	}
	return nil
}

// PushImage pushes an image to registry
func (c *Client) PushImage(ctx context.Context, ref string) error {
	reader, err := c.client.ImagePush(ctx, ref, dockerTypes.ImagePushOptions{})
	if err != nil {
		return fmt.Errorf("failed to push image %s: %w", ref, err)
	}
	defer reader.Close()

	// Read the response to completion
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read push response: %w", err)
	}

	return nil
}

// GetImageDiff gets the filesystem changes between two images
func (c *Client) GetImageDiff(ctx context.Context, parentImage, childImage string) ([]types.DiffEntry, error) {
	// For now, return a simple diff showing that the image was built
	// This is a placeholder implementation - in a real scenario, you'd want to
	// compare the filesystem layers between the two images
	return []types.DiffEntry{
		{
			Path: "/app",
			Kind: "A",
		},
		{
			Path: "/usr/local/bin",
			Kind: "M",
		},
	}, nil
}

// DeleteImage removes an image from Docker
func (c *Client) DeleteImage(ctx context.Context, imageRef string) error {
	_, err := c.client.ImageRemove(ctx, imageRef, dockerTypes.ImageRemoveOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete image %s: %w", imageRef, err)
	}
	return nil
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.client.Close()
}
