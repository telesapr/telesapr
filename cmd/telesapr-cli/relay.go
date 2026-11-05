package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// This file is the tool as the relay to the browser version: the
// TeleSAPR extension (cmd/telesapr-web/extension) starts it by native
// messaging, and it listens on a socket as the window does, so that
// telesapr-cli mcp reaches the drawing open in the page
// (agent.Relay). --install tells the browsers where it is.

const relayUsage = `    browser-relay
        The relay between mcp and the browser version (Try in browser): the
        TeleSAPR extension starts it while a TeleSAPR page is open, and mcp
        then reaches the drawing in the page as it reaches the window's.
        It is not run by hand.
    browser-relay --install [--browser <name>] [--extension-id <id>]
    browser-relay --uninstall [--browser <name>]
        Tells the browsers where the relay is, or that it is gone. <name> is
        chrome, chromium or firefox; with none, every one of them that is
        installed for you. <id> is the extension's, if it is not the one
        the extension in TeleSAPR's sources has (doc/guide/ai.md).
`

// relayHost is the name the extension starts the relay by.
const relayHost = "org.telesapr.relay"

// relayExtensionID is the Chrome extension's id, fixed by the key in
// its manifest; relayGeckoID is the Firefox extension's.
const (
	relayExtensionID = "phfjimhbnbiipcgepfkplfehmfmoejjh"
	relayGeckoID     = "relay@telesapr.org"
)

// startedByBrowser is whether a browser started the tool for its
// extension: Chrome gives the extension's origin, Firefox the host's
// manifest and the extension's id.
func startedByBrowser(args []string) bool {
	if len(args) >= 2 && strings.HasPrefix(args[1], "chrome-extension://") {
		return true
	}
	return len(args) >= 3 && args[2] == relayGeckoID
}

// runRelay performs the browser-relay command, or carries the
// connections for a browser that started the tool, and returns the
// exit status.
func runRelay(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if startedByBrowser(args) {
		return relay(stdin, stdout, stderr)
	}
	install, uninstall, browser, id := false, false, "", relayExtensionID
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		switch arg := rest[i]; arg {
		case "--install":
			install = true
		case "--uninstall":
			uninstall = true
		case "--browser", "--extension-id":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintf(stderr, "%s needs a value.\n", arg)
				return exitUsage
			}
			i++
			if arg == "--browser" {
				browser = rest[i]
			} else {
				id = rest[i]
			}
		default:
			_, _ = fmt.Fprintf(stderr, "Unrecognized option '%s'.\n", arg)
			return exitUsage
		}
	}
	switch {
	case install && uninstall:
		_, _ = fmt.Fprintln(stderr, "--install and --uninstall go one at a time.")
		return exitUsage
	case !install && !uninstall:
		return relay(stdin, stdout, stderr)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	targets, err := relayTargets(runtime.GOOS, home, browser)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s\n", capitalized(err.Error()))
		return exitUsage
	}
	if uninstall {
		for _, t := range targets {
			path := filepath.Join(t.dir, relayHost+".json")
			if err := os.Remove(path); err == nil {
				_, _ = fmt.Fprintf(stdout, "Removed %s\n", path)
			} else if !errors.Is(err, os.ErrNotExist) {
				_, _ = fmt.Fprintln(stderr, err)
				return 1
			}
		}
		return exitOK
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	for _, t := range targets {
		path, err := writeRelayManifest(t, exe, id)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "Wrote %s\n", path)
		if runtime.GOOS == "windows" {
			_, _ = fmt.Fprintf(stdout, "Then, for %s: reg add \"%s\" /ve /t REG_SZ /d \"%s\" /f\n", t.browser, t.registry, path)
		}
	}
	return exitOK
}

// relay carries the connections to the extension on stdin and stdout.
func relay(stdin io.Reader, stdout, stderr io.Writer) int {
	dir, err := agent.SocketDir()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	l, path, err := agent.Listen(dir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	// Standard output is the extension's; the browser keeps this.
	_, _ = fmt.Fprintf(stderr, "telesapr-cli browser-relay: listening at %s\n", path)
	if err := agent.Relay(l, stdin, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return exitOK
}

// relayTarget is where a browser looks for the relay's manifest.
type relayTarget struct {
	browser string
	dir     string
	// firefox is whether the manifest names the extension as Firefox
	// does; registry is the key Windows has the manifest's path in.
	firefox  bool
	registry string
}

// relayTargets are the browsers to install the relay for: browser, or
// with "" each whose folder for the user is there.
func relayTargets(goos, home, browser string) ([]relayTarget, error) {
	var all []relayTarget
	switch goos {
	case "linux", "freebsd", "openbsd", "netbsd":
		all = []relayTarget{
			{browser: "chrome", dir: filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts")},
			{browser: "chromium", dir: filepath.Join(home, ".config", "chromium", "NativeMessagingHosts")},
			{browser: "firefox", dir: filepath.Join(home, ".mozilla", "native-messaging-hosts"), firefox: true},
		}
	case "darwin":
		support := filepath.Join(home, "Library", "Application Support")
		all = []relayTarget{
			{browser: "chrome", dir: filepath.Join(support, "Google", "Chrome", "NativeMessagingHosts")},
			{browser: "chromium", dir: filepath.Join(support, "Chromium", "NativeMessagingHosts")},
			{browser: "firefox", dir: filepath.Join(support, "Mozilla", "NativeMessagingHosts"), firefox: true},
		}
	case "windows":
		// Windows finds the manifest through the registry; it may be
		// anywhere, so it goes beside the tool's sockets.
		dir, err := agent.SocketDir()
		if err != nil {
			return nil, err
		}
		key := `HKCU\Software\%s\NativeMessagingHosts\` + relayHost
		all = []relayTarget{
			{browser: "chrome", dir: filepath.Join(dir, "chrome"), registry: fmt.Sprintf(key, `Google\Chrome`)},
			{browser: "chromium", dir: filepath.Join(dir, "chromium"), registry: fmt.Sprintf(key, "Chromium")},
			{browser: "firefox", dir: filepath.Join(dir, "firefox"), firefox: true, registry: fmt.Sprintf(key, "Mozilla")},
		}
	default:
		return nil, fmt.Errorf("browsers on %s are not known to the relay", goos)
	}
	if browser != "" {
		for _, t := range all {
			if t.browser == browser {
				return []relayTarget{t}, nil
			}
		}
		return nil, fmt.Errorf("%q is not a browser the relay knows: chrome, chromium or firefox", browser)
	}
	var found []relayTarget
	for _, t := range all {
		// The folder the manifests go in is the browser's own folder's.
		if _, err := os.Stat(filepath.Dir(t.dir)); err == nil || goos == "windows" {
			found = append(found, t)
		}
	}
	if len(found) == 0 {
		return nil, errors.New("none of chrome, chromium and firefox is installed for you; name one with --browser")
	}
	return found, nil
}

// writeRelayManifest writes the manifest by which t's browser starts
// exe for the extension with id, and returns its path.
func writeRelayManifest(t relayTarget, exe, id string) (string, error) {
	m := map[string]any{
		"name":        relayHost,
		"description": "TeleSAPR: the relay between telesapr-cli mcp and the browser version",
		"path":        exe,
		"type":        "stdio",
	}
	if t.firefox {
		m["allowed_extensions"] = []string{relayGeckoID}
	} else {
		m["allowed_origins"] = []string{"chrome-extension://" + id + "/"}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(t.dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(t.dir, relayHost+".json")
	return path, os.WriteFile(path, append(data, '\n'), 0o644)
}
