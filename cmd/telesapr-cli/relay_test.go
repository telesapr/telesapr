package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The browsers start the tool as their native messaging hosts, each
// in its own way.
func TestStartedByBrowser(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"telesapr-cli", "chrome-extension://" + relayExtensionID + "/"}, true},
		{[]string{"telesapr-cli", "chrome-extension://" + relayExtensionID + "/", "--parent-window=0"}, true},
		{[]string{"telesapr-cli", "/home/u/.mozilla/native-messaging-hosts/org.telesapr.relay.json", relayGeckoID}, true},
		{[]string{"telesapr-cli", "mcp"}, false},
		{[]string{"telesapr-cli", "a.tlspr", "b.stl"}, false},
		{[]string{"telesapr-cli"}, false},
	} {
		if got := startedByBrowser(c.args); got != c.want {
			t.Errorf("startedByBrowser(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}

// --install writes a manifest for each browser there is, naming the
// tool and the extension, and --uninstall takes them away. It sets
// HOME, so it can't run beside the others.
func TestRelayInstall(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the folders checked are Linux's")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "google-chrome"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".mozilla"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if status := runRelay([]string{"telesapr-cli", "browser-relay", "--install"}, nil, &out, &errs); status != exitOK {
		t.Fatalf("--install = %d: %s", status, errs.String())
	}
	chrome := filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts", relayHost+".json")
	firefox := filepath.Join(home, ".mozilla", "native-messaging-hosts", relayHost+".json")
	chromium := filepath.Join(home, ".config", "chromium", "NativeMessagingHosts", relayHost+".json")
	read := func(path string) map[string]any {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	c := read(chrome)
	if c["name"] != relayHost || c["path"] != exe || c["type"] != "stdio" {
		t.Errorf("Chrome's manifest = %v", c)
	}
	if o, _ := c["allowed_origins"].([]any); len(o) != 1 || o[0] != "chrome-extension://"+relayExtensionID+"/" {
		t.Errorf("Chrome's manifest allows %v", c["allowed_origins"])
	}
	if e, _ := read(firefox)["allowed_extensions"].([]any); len(e) != 1 || e[0] != relayGeckoID {
		t.Errorf("Firefox's manifest allows %v", e)
	}
	if _, err := os.Stat(chromium); err == nil {
		t.Error("a manifest was written for Chromium, which isn't installed")
	}

	out.Reset()
	if status := runRelay([]string{"telesapr-cli", "browser-relay", "--install", "--browser", "chromium", "--extension-id", "abc"}, nil, &out, &errs); status != exitOK {
		t.Fatalf("--install --browser chromium = %d: %s", status, errs.String())
	}
	if o, _ := read(chromium)["allowed_origins"].([]any); len(o) != 1 || o[0] != "chrome-extension://abc/" {
		t.Errorf("--extension-id abc allows %v", o)
	}

	if status := runRelay([]string{"telesapr-cli", "browser-relay", "--uninstall"}, nil, &out, &errs); status != exitOK {
		t.Fatalf("--uninstall = %d: %s", status, errs.String())
	}
	for _, path := range []string{chrome, firefox} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s is still there after --uninstall", path)
		}
	}

	for _, args := range [][]string{
		{"--install", "--uninstall"},
		{"--install", "--browser", "netscape"},
		{"--browser"},
		{"--frobnicate"},
	} {
		errs.Reset()
		if status := runRelay(append([]string{"telesapr-cli", "browser-relay"}, args...), nil, &out, &errs); status != exitUsage {
			t.Errorf("browser-relay %q = %d, want %d", args, status, exitUsage)
		}
	}
}

// Started by a browser, the tool listens until the browser hangs up.
// It sets XDG_RUNTIME_DIR, where the socket goes.
func TestRelayStartedByBrowser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the socket's folder is set through XDG_RUNTIME_DIR")
	}
	dir, err := os.MkdirTemp("", "rl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("TMPDIR", dir)
	var out, errs bytes.Buffer
	status := runRelay([]string{"telesapr-cli", "chrome-extension://" + relayExtensionID + "/"}, strings.NewReader(""), &out, &errs)
	if status != exitOK {
		t.Fatalf("the relay = %d: %s", status, errs.String())
	}
	if out.Len() != 0 {
		t.Errorf("the relay wrote %q to the extension with nothing connected", out.String())
	}
	if !strings.Contains(errs.String(), "listening at") {
		t.Errorf("the relay said %q", errs.String())
	}
}
