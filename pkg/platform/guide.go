package platform

import "io/fs"

// GuideHost is a Platform that shows the user's guide in a window of its
// own; one without it opens the guide's site in a browser.
type GuideHost interface {
	// ShowGuide shows a page of pages (a Markdown file, as "fillet.md"),
	// opening the guide's window if it is closed. online is the guide's
	// site, which the window opens for what it cannot show itself, such
	// as a video.
	ShowGuide(pages fs.FS, page, online string)
}
