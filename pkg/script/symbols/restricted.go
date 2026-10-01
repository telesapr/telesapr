package symbols

import (
	"errors"
	"os"
	"strconv"
)

// Exit is what a script's os.Exit panics with instead of ending the
// program, which may be the application with a drawing open.
type Exit int

func (e Exit) Error() string { return "os.Exit(" + strconv.Itoa(int(e)) + ")" }

// osExit stands in for os.Exit in a script.
func osExit(code int) { panic(Exit(code)) }

var errRestricted = errors.New("restricted")

// osFindProcess stands in for os.FindProcess in a script: it won't
// find this process.
func osFindProcess(pid int) (*os.Process, error) {
	if pid == os.Getpid() {
		return nil, errRestricted
	}
	return os.FindProcess(pid)
}
