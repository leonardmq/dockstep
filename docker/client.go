package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

	"dockstep.dev/types"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Client wraps the Docker client with dockstep-specific functionality
type Client struct {
	client *client.Client
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

// CreateContainer creates a container from an image
func (c *Client) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
	// Convert dockstep config to Docker config
	containerConfig := &container.Config{
		Image:           config.Image,
		Cmd:             []string{"/bin/sh", "-c", config.Cmd},
		WorkingDir:      config.Workdir,
		Env:             config.Env,
		NetworkDisabled: config.Network == types.NetworkNone,
	}

	hostConfig := &container.HostConfig{
		Binds:       config.Mounts,
		NetworkMode: container.NetworkMode(config.Network),
	}

	if config.Resources != nil {
		// Parse CPU and memory limits
		if config.Resources.CPU != "" {
			// Docker expects CPU quota in microseconds
			// This is a simplified implementation
			hostConfig.CPUQuota = 100000 // Default 1 CPU
		}
		if config.Resources.Memory != "" {
			// Parse memory limit (e.g., "512m", "1g")
			// This is a simplified implementation
			hostConfig.Memory = 512 * 1024 * 1024 // Default 512MB
		}
	}

	resp, err := c.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer starts a container
func (c *Client) StartContainer(ctx context.Context, id string) error {
	err := c.client.ContainerStart(ctx, id, dockerTypes.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container %s: %w", id, err)
	}
	return nil
}

// WaitContainer waits for a container to finish and returns the exit code
func (c *Client) WaitContainer(ctx context.Context, id string) (int, error) {
	statusCh, errCh := c.client.ContainerWait(ctx, id, container.WaitConditionNotRunning)

	select {
	case status := <-statusCh:
		return int(status.StatusCode), nil
	case err := <-errCh:
		return -1, fmt.Errorf("failed to wait for container %s: %w", id, err)
	}
}

// GetLogs streams container logs to a writer. If follow is true, it follows until the container stops.
func (c *Client) GetLogs(ctx context.Context, id string, w io.Writer, follow bool) error {
	reader, err := c.client.ContainerLogs(ctx, id, dockerTypes.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
	})
	if err != nil {
		return fmt.Errorf("failed to get logs for container %s: %w", id, err)
	}
	defer reader.Close()

	_, err = io.Copy(w, reader)
	if err != nil {
		return fmt.Errorf("failed to copy logs: %w", err)
	}

	return nil
}

// CommitContainer commits a container to an image
func (c *Client) CommitContainer(ctx context.Context, id, tag string) (string, error) {
	resp, err := c.client.ContainerCommit(ctx, id, dockerTypes.ContainerCommitOptions{
		Reference: tag,
	})
	if err != nil {
		return "", fmt.Errorf("failed to commit container %s: %w", id, err)
	}

	return resp.ID, nil
}

// DiffContainer gets filesystem changes for a container
func (c *Client) DiffContainer(ctx context.Context, id string) ([]types.DiffEntry, error) {
	changes, err := c.client.ContainerDiff(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff for container %s: %w", id, err)
	}

	var entries []types.DiffEntry
	for _, change := range changes {
		entry := types.DiffEntry{
			Path: change.Path,
		}

		switch change.Kind {
		case 0: // Add
			entry.Kind = "A"
		case 1: // Delete
			entry.Kind = "D"
		case 2: // Modify
			entry.Kind = "M"
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	err := c.client.ContainerRemove(ctx, id, dockerTypes.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove container %s: %w", id, err)
	}
	return nil
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

// CopyToContainer copies content to a container
func (c *Client) CopyToContainer(ctx context.Context, containerID string, destPath string, content io.Reader) error {
	return c.client.CopyToContainer(ctx, containerID, destPath, content, dockerTypes.CopyToContainerOptions{})
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.client.Close()
}

// ContainerConfig represents configuration for creating a container
type ContainerConfig struct {
	Image     string
	Cmd       string
	Workdir   string
	Env       []string
	Mounts    []string
	Network   types.NetworkMode
	Resources *types.Resources
}
