// Package migrations embeds the SQL migration files so the binary is
// self-contained and tests can migrate a scratch database without shelling out.
package migrations

import "embed"

// FS holds every migration in lexical order (0001_..., 0002_...).
//
//go:embed *.sql
var FS embed.FS
