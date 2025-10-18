package assets

import "embed"

// UI holds the built SPA assets when present at build time under ui/dist.
//
//go:embed ui/dist/*
var UI embed.FS
