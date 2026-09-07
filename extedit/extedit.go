// Package extedit hands multi-line text entry to the user's real editor.
// The TUI suspends, $EDITOR opens on a temp file, and the saved contents
// come back — the k9s/git-commit pattern. Workbench TUIs use this for JSON
// profiles and anything longer than one line; short fields stay plain
// tview inputs.
package extedit

import (
	"fmt"
	"os"
	"os/exec"
)

// Suspender is the slice of *tview.Application this package needs; tests
// substitute a fake that runs the function inline.
type Suspender interface {
	Suspend(f func()) bool
}

// Edit suspends the app and opens the user's editor ($EDITOR, else $VISUAL,
// else vi) on a temp file seeded with initial. pattern names the temp file
// (os.CreateTemp semantics — include an extension, e.g. "profile-*.json",
// so the editor picks the right filetype). Returns the file's contents at
// editor exit; a non-zero editor exit aborts and returns an error.
func Edit(app Suspender, pattern string, initial []byte) ([]byte, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, fmt.Errorf("extedit: %w", err)
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.Write(initial); err != nil {
		f.Close()
		return nil, fmt.Errorf("extedit: seed %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("extedit: %w", err)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	var runErr error
	if ok := app.Suspend(func() {
		// sh -c keeps multi-word EDITOR values ("code --wait") working; the
		// path is passed as a positional argument, not interpolated.
		cmd := exec.Command("sh", "-c", editor+` "$1"`, "extedit", path)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		runErr = cmd.Run()
	}); !ok {
		return nil, fmt.Errorf("extedit: application not running")
	}
	if runErr != nil {
		return nil, fmt.Errorf("extedit: %s: %w", editor, runErr)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("extedit: read back: %w", err)
	}
	return out, nil
}
