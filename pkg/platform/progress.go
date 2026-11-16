package platform

// Progress is a window that shows how far a long operation run on the
// loop's goroutine (opening a file, exporting one) has got. The work
// keeps the loop, so nothing else the program does runs meanwhile:
// Update is where the platform lets its windows repaint, and what comes
// from the user then waits or is dropped, as during a modal dialog.
type Progress interface {
	// Update shows text and the fraction of the work done, from 0 to 1;
	// a fraction below 0 is one not known. It may be called often: the
	// platform repaints only now and then.
	Update(text string, fraction float64)
	// Close takes the window away and gives the program back what it
	// held while the window was up.
	Close()
}

// ProgressShower is a Platform that can show a Progress window over
// parent. A platform that doesn't implement it shows none.
type ProgressShower interface {
	ShowProgress(parent Window, title string) Progress
}
