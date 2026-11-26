// Package guide holds the user's guide, its pages and pictures, for the
// window to show from the Help menu without a network, each video as the
// animated GIF beside it. The WebM and MP4 videos, the scenarios that
// make the pictures and the site's stylesheets stay out: the site plays
// the videos.
package guide

import "embed"

// Pages are the guide's Markdown pages, their pictures and the GIFs of
// their videos, at the paths the pages link them by; README.md is the first page and nav.yml the
// contents, a copy of the site's.
//
//go:embed *.md nav.yml reference/*.md images/*.png videos/*.gif
var Pages embed.FS
