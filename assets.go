// Package isppr embeds repo-level assets (SQL migrations, demo seed) so the
// API binary can apply them at boot without external file paths.
package isppr

import "embed"

//go:embed migrations/*.sql seed/*.sql templates/base/*
var Assets embed.FS
