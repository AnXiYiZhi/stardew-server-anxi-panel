package migrations

import "embed"

// Files contains SQL migrations applied by the backend at startup.
//
//go:embed *.sql
var Files embed.FS
