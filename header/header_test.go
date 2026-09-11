package header

import (
	"strings"
	"testing"

	"github.com/aceagles/tui-toolkit/theme"
)

func TestIsLoopback(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1": true, "127.0.0.2": true, "127.255.255.254": true,
		"localhost": true, "LocalHost": true, "::1": true, "[::1]": true,
		"127.0.0.1:502": true, "[::1]:502": true, "localhost:8080": true,
		"192.0.2.1": false, "10.0.0.1": false, "128.0.0.1": false, "::2": false,
		"example.com": false, "192.0.2.1:502": false, "": false,
	} {
		if got := IsLoopback(host); got != want {
			t.Errorf("IsLoopback(%q) = %v, want %v", host, got, want)
		}
	}
}

// A remote target is plain foreground text in RoleRemote: no background,
// no banner word, one line.
func TestRenderTargets(t *testing.T) {
	theme.Apply(theme.Night)
	p := theme.Night
	remote := Render(Info{Tool: "tool", Targets: []Target{{Addr: "tcp://192.0.2.1:502", Remote: true}}, Plane: "tcp"})
	if !strings.Contains(remote, p.Tag(theme.RoleRemote)+"tcp://192.0.2.1:502[-]") {
		t.Errorf("remote target not in RoleRemote: %q", remote)
	}
	if strings.Contains(remote, "REMOTE") || strings.Contains(remote, ":#") {
		t.Errorf("remote target has a banner or background: %q", remote)
	}
	local := Render(Info{Tool: "tool", Targets: []Target{{Addr: "tcp://127.0.0.1:502"}}, Plane: "tcp"})
	if strings.Contains(local, p.Hex(theme.RoleRemote)) {
		t.Errorf("loopback target coloured remote: %q", local)
	}
	two := Render(Info{Targets: []Target{{Addr: "a"}, {Addr: "b", Remote: true}}})
	if !strings.HasPrefix(two, "a  "+p.Tag(theme.RoleRemote)+"b[-]") {
		t.Errorf("two targets: %q", two)
	}
	for _, h := range []string{remote, local, two} {
		if strings.Contains(h, "\n") {
			t.Errorf("header %q is more than one line", h)
		}
	}
}

func TestRenderConnAndFields(t *testing.T) {
	theme.Apply(theme.Night)
	p := theme.Night
	for _, c := range []struct {
		conn   Conn
		detail string
		want   string
	}{
		{ConnConnecting, "", p.Tag(theme.RoleWarn) + "connecting[-]"},
		{ConnUp, "", p.Tag(theme.RoleOK) + "connected[-]"},
		{ConnDown, "refused [x]", p.Tag(theme.RoleError) + "disconnected: refused [x[][-]"},
	} {
		got := Render(Info{Conn: c.conn, ConnDetail: c.detail})
		if want := p.Tag(theme.RoleFgMuted) + "link[-] " + c.want; got != want {
			t.Errorf("conn %d: %q, want %q", c.conn, got, want)
		}
	}
	if got := Render(Info{Tool: "t"}); strings.Contains(got, "link") {
		t.Errorf("ConnNone rendered a link state: %q", got)
	}
	got := Render(Info{Fields: []Field{{Label: "unit", Value: "[::b]1[::-]"}}, Pending: " g "})
	want := p.Tag(theme.RoleFgMuted) + "unit[-] [::b]1[::-]   " + p.Tag(theme.RoleAccent) + "[::b]<space>g[-::-]"
	if got != want {
		t.Errorf("fields and pending:\n%q\nwant\n%q", got, want)
	}
}
