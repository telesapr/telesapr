//go:build js && wasm

package web

import (
	"encoding/json"
	"syscall/js"
)

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

// jsonString is v as JSON text, for the page to parse.
func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // a programmer error: the values are plain structs
	}
	return string(b)
}

// localStore is the page's localStorage as a store. Storage that is
// full or refused (a private window) keeps nothing, and the settings
// stay as they were.
type localStore struct{ s js.Value }

func (l localStore) get(key string) (string, bool) {
	v := l.s.Call("getItem", key)
	if v.IsNull() || v.IsUndefined() {
		return "", false
	}
	return v.String(), true
}

func (l localStore) set(key, value string) {
	defer func() { _ = recover() }() // setItem throws when the quota is used up; js.Error is a panic here
	l.s.Call("setItem", key, value)
}
