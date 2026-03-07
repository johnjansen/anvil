package tools

import "embed"

// FS contains the embedded tools directory contents.
// Everything under tools/ (except this file) gets embedded.
//
//go:embed all:skills
var FS embed.FS
