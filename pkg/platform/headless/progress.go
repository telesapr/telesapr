package headless

import "github.com/telesapr/telesapr/pkg/platform"

var _ platform.ProgressShower = (*Platform)(nil)

// Progress is a progress window, recorded: its title, every update and
// whether it was closed.
type Progress struct {
	Title   string
	Updates []ProgressUpdate
	Closed  bool
	// DialogsBefore and FormsBefore are how many message and form
	// dialogs had been shown, and FileDialogsBefore how many file
	// dialogs made, when it was shown: those were all closed by then,
	// since each of them runs to its end before it returns.
	DialogsBefore, FormsBefore, FileDialogsBefore int
}

// ProgressUpdate is one Update of a Progress.
type ProgressUpdate struct {
	Text     string
	Fraction float64
}

// ShowProgress records a progress window.
func (p *Platform) ShowProgress(_ platform.Window, title string) platform.Progress {
	pr := &Progress{Title: title, DialogsBefore: len(p.DialogsShown), FormsBefore: len(p.FormsShown),
		FileDialogsBefore: len(p.FileDialogs)}
	p.ProgressShown = append(p.ProgressShown, pr)
	return pr
}

// Update records text and fraction.
func (pr *Progress) Update(text string, fraction float64) {
	pr.Updates = append(pr.Updates, ProgressUpdate{text, fraction})
}

// Close records that the window was closed.
func (pr *Progress) Close() { pr.Closed = true }
