package ui

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode"

	"github.com/telesapr/telesapr/pkg/platform"
)

// messageBox shows a message: the first sentence (up to a '.' or ':'
// followed by white space) as the message, the rest as its description
// (C++ MessageBox, util.cpp).
func (ss *Application) messageBox(text string, isError bool, onDismiss func()) {
	if ss.messageSink != nil {
		// A script is running: a dialog would run the event loop under
		// it, so the script takes the message.
		ss.messageSink(text, isError)
		if onDismiss != nil {
			onDismiss()
		}
		return
	}
	separatorAt := 0
	found := -1
	for {
		dotAt := strings.IndexByte(text[min(separatorAt+1, len(text)):], '.')
		colonAt := strings.IndexByte(text[min(separatorAt+1, len(text)):], ':')
		next := -1
		for _, at := range []int{dotAt, colonAt} {
			if at >= 0 && (next < 0 || at < next) {
				next = at
			}
		}
		if next < 0 {
			found = -1
			break
		}
		separatorAt = separatorAt + 1 + next
		if separatorAt+1 < len(text) && isSpace(text[separatorAt+1]) {
			found = separatorAt
			break
		}
	}
	message := text
	description := ""
	if found >= 0 {
		message = text[:found+1]
		if found+1 < len(text) {
			description = strings.TrimLeftFunc(text[found+1:], unicode.IsSpace)
		}
	}

	var dialog platform.MessageDialog
	if ss.Platform != nil {
		dialog = ss.Platform.CreateMessageDialog(ss.GW.window)
	}
	if dialog == nil {
		if isError {
			log.Printf("Error: %s", message)
		} else {
			log.Printf("Message: %s", message)
		}
		if onDismiss != nil {
			onDismiss()
		}
		return
	}
	if isError {
		dialog.SetType(platform.DialogError)
		dialog.SetTitle("Error")
	} else {
		dialog.SetType(platform.DialogInformation)
		dialog.SetTitle("Message")
	}
	dialog.SetMessage(message)
	if description != "" {
		dialog.SetDescription(description)
	}
	dialog.AddButton("&OK", platform.ResponseOK, true)
	dialog.RunModal()
	if onDismiss != nil {
		onDismiss()
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// Error shows an error message (C++ Error).
func (ss *Application) Error(format string, args ...any) {
	ss.messageBox(fmt.Sprintf(format, args...), true, nil)
}

// ErrDeclined is the error of an action the user gave up on at one of
// its dialogs: nothing to report.
var ErrDeclined = errors.New("declined")

// Report shows err to the user, unless there is none or the user
// declined the action themselves.
func (ss *Application) Report(err error) {
	if err == nil || errors.Is(err, ErrDeclined) {
		return
	}
	ss.Error("%s", sentence(err.Error()))
}

// sentence is an error's text as a message: capitalised, and ending
// with a full stop unless it ends with other punctuation.
func sentence(s string) string {
	s = capitalize(strings.TrimRight(s, " \n"))
	if s == "" || strings.ContainsRune(".!?:", rune(s[len(s)-1])) {
		return s
	}
	return s + "."
}

// capitalize starts a message with a capital, as the dialogs do.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Message shows an informational message (C++ Message).
func (ss *Application) Message(format string, args ...any) {
	ss.messageBox(fmt.Sprintf(format, args...), false, nil)
}

// MessageAndRun shows a message, then runs onDismiss (C++
// MessageAndRun).
func (ss *Application) MessageAndRun(onDismiss func(), format string, args ...any) {
	ss.messageBox(fmt.Sprintf(format, args...), false, onDismiss)
}
