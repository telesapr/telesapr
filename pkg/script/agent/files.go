package agent

import (
	"os"

	"github.com/telesapr/telesapr/pkg/script"
)

// GuardFiles is the guard of the files of an AI program's scripts
// (script.Files): they may read and write in the document's folder, as
// folder gives it at each check ("" for none); in the folders given;
// and in a temporary folder of their own, which done removes. They may read the script library. Nothing else:
// a script reads and writes files with its user's permissions, and the
// program that writes it isn't the user.
func GuardFiles(folder func() string, folders []string, library string) (files *script.Files, done func(), err error) {
	temp, err := os.MkdirTemp("", "telesapr-ai-")
	if err != nil {
		return nil, nil, err
	}
	files = &script.Files{
		Write:  folders,
		Folder: folder,
		Temp:   temp,
	}
	if library != "" {
		files.Read = []string{library}
	}
	return files, func() { _ = os.RemoveAll(temp) }, nil
}
