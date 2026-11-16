# The progress window

Refreshed 2026-11-16.

Opening a file and exporting one show a window with a line of text and a bar, from when the
dialogs that chose the file have closed until the work ends. `pkg/ui/progress.go` starts it
(`withProgress`, around `Application.Load`, the File > Export entries, Import and the drawing's
export) and feeds it: a phase ("Reading", "Writing") with no fraction, and the group a
regeneration begins (`appHooks.Regenerating`) with its fraction. The platforms show it through
`platform.ProgressShower`; headless records it (`headless.Progress`), the browser shows it in
its busy box (`browser.md`).

## The desktop window (`pkg/platform/tkgui/progress.go`)

The work keeps the loop's goroutine, so the window only repaints when the work lets the loop run.
`Update` does so every 100 ms, for 5 ms, in takigo's nested loop, as a modal dialog does; showing
the window runs it until the window is mapped (300 ms at most) and 30 ms more.

- A nested loop runs everything that is queued, and the model is half made: the application's
  timers (the refresh, the autosave), the work posted to the loop (`PostToLoop`, scripts from AI
  programs), a window's close and its renders are held (`Platform.runOrHold`) and run in order
  once the window closes. A held timer that is started again fires once (`Timer.gen`).
- Input: a local grab on the window and the focus, as a dialog takes; a busy cover over every
  toplevel of the program on screen (the main window, the property browser, the console) and the
  window's own cursor give the watch. The window manager's close button does nothing.
- Its size is fixed, so that it doesn't jump with its text, and made from the label's font (56
  characters by two lines, the text wrapped): a size in pixels cut the text off on a desktop
  with a large font (2026-11-16).
- A dialog the work asks (a missing linked file, a warning) hides the window while it runs
  (`progressAside`, called by every `RunModal`) and shows it again.
- Renders are held only from when `ShowProgress` returns: before, the model is still whole, and
  the GL windows render once so that a copy of their frame is kept (`canvas.presentGL`). A GL
  window has no frame to copy back on an exposure; without the copy, what a closed dialog
  covered stayed white for the whole of the work. The software renderer keeps its frame anyway.
- Closing runs the loop once more before releasing what was held: the held refresh regenerates
  and took 1.2 s before the uncovered part of the main window was painted.
- A window raised while it is up, as the main window is when it is first mapped during an open
  from the command line, puts the progress window back over it (`Window.present`).

## Not done

- Nothing repaints while a single step runs long: reading the links of a large assembly (about
  2 s for 50 linked parts), one group's Boolean, writing the file. `sketch.KernelPacer` would
  pace inside a Boolean, but paced loops run in order, so the desktop does not use it.
- A regeneration that makes two passes over the groups fills the bar twice.
- No cancel button: the work has no way to stop between its steps except a regeneration's
  `Cancel`, and stopping an open half way leaves no model.
- Saving, linking a part and the bill of materials and trace CSVs show no window.
