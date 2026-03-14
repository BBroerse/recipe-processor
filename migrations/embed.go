// Package migrations embeds all SQL migration files so they are compiled
// into the binary and available at runtime without filesystem access.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
