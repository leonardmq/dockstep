package export

import (
	"context"
	"fmt"

	"dockstep.dev/docker"
	"dockstep.dev/store"
	"dockstep.dev/types"
)

// TagImage tags and optionally pushes an image
func TagImage(ctx context.Context, dockerClient *docker.Client, store *store.Store, blockID string, opts types.ImageExportOptions) error {
	// Get the image digest for the block
	digest, err := store.LoadImageDigest(blockID)
	if err != nil {
		return fmt.Errorf("failed to load image digest for block %s: %w", blockID, err)
	}

	if digest == "" {
		return fmt.Errorf("no image digest found for block %s", blockID)
	}

	// Tag the image
	if err := dockerClient.TagImage(ctx, digest, opts.Tag); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}

	fmt.Printf("Tagged image as %s\n", opts.Tag)

	// Push if requested
	if opts.Push {
		if err := dockerClient.PushImage(ctx, opts.Tag); err != nil {
			return fmt.Errorf("failed to push image: %w", err)
		}
		fmt.Printf("Pushed image %s\n", opts.Tag)
	}

	return nil
}
