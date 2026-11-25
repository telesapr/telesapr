// Package guide holds the user's guide, its pages and pictures, for the
// window to show from the Help menu without a network. The videos, the
// scenarios that make the pictures and the site's stylesheets stay out:
// a video is shown on the guide's site.
package guide

import "embed"

// Pages are the guide's Markdown pages and their pictures, at the paths
// the pages link them by; README.md is the first page and nav.yml the
// contents, a copy of the site's.
//
//go:embed *.md nav.yml reference/*.md images/*.png
var Pages embed.FS
