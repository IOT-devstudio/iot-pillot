// Package migrations embeds the same SQL used for local and production migrations.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
