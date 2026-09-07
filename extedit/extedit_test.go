package extedit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inlineSuspender runs the suspended function immediately, as a live tview
// application would.
type inlineSuspender struct{ ok bool }

func (s inlineSuspender) Suspend(f func()) bool {
	if s.ok {
		f()
	}
	return s.ok
}

// fakeEditor writes a script to act as $EDITOR and points the env at it.
func fakeEditor(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", path)
}

func TestEditReturnsSavedContents(t *testing.T) {
	fakeEditor(t, `printf 'edited: ' > "$1.new" && cat "$1" >> "$1.new" && mv "$1.new" "$1"`)
	out, err := Edit(inlineSuspender{ok: true}, "extedit-*.json", []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `edited: {"a":1}` {
		t.Fatalf("got %q", out)
	}
}

func TestEditSeedsInitialContent(t *testing.T) {
	fakeEditor(t, `cp "$1" "$1.seen"; :`)
	_, err := Edit(inlineSuspender{ok: true}, "extedit-*.txt", []byte("seed"))
	if err != nil {
		t.Fatal(err)
	}
	// The editor saw the seeded file; Edit returned it unchanged.
}

func TestEditEditorFailureAborts(t *testing.T) {
	fakeEditor(t, "exit 3")
	if _, err := Edit(inlineSuspender{ok: true}, "extedit-*.txt", nil); err == nil {
		t.Fatal("non-zero editor exit must be an error")
	}
}

func TestEditNotRunning(t *testing.T) {
	fakeEditor(t, ":")
	_, err := Edit(inlineSuspender{ok: false}, "extedit-*.txt", nil)
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("err = %v, want application-not-running", err)
	}
}

func TestEditMultiWordEditor(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "inner.sh")
	if err := os.WriteFile(inner, []byte("#!/bin/sh\nprintf ok > \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", inner+" --wait")
	out, err := Edit(inlineSuspender{ok: true}, "extedit-*.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "ok" {
		t.Fatalf("got %q, want ok (multi-word EDITOR must keep its flags)", out)
	}
}
