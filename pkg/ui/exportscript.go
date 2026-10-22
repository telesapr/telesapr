package ui

import (
	"strings"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// ExportScriptTo writes the model as a script that builds it again, as
// File > Export > Script does, through the call a script makes
// (File.ExportScript). What the script can't write yet is said in a
// message.
func (ss *Application) ExportScriptTo(path string) error {
	notes, err := api.NewDoc(ss.ScriptHost().onLoop()).File.ExportScript(path)
	if err != nil {
		return err
	}
	if len(notes) > 0 {
		ss.Message("The script leaves out what it can't write yet, "+
			"each said in a comment there:\n\n%s", strings.Join(notes, "\n"))
	}
	return nil
}
