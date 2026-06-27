package static

import "embed"

// FS holds the frontend build output (index.html + assets/).
// In production (Docker build), the Dockerfile copies frontend/dist/* here
// before the Go binary is compiled. In local development this is empty;
// developers should use the Vite dev server (npm run dev) instead.
//
//go:embed frontend_dist/*
var FS embed.FS
