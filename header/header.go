// Package header renders the one-line header a workbench TUI keeps at the
// top of the screen: tool name, target(s), plane, extra fields and link
// state, so the operator always sees what the keys will act on.
//
// A target off this machine is drawn in theme.RoleRemote as plain text: no
// background fill, no banner. IsLoopback decides which targets are local.
//
//	view.SetText(header.Render(header.Info{
//		Tool:    "mytui",
//		Targets: []header.Target{{Addr: url, Remote: !header.IsLoopback(host)}},
//		Plane:   "http",
//		Conn:    header.ConnUp,
//	}))
package header

import (
	"fmt"
	"net"
	"strings"

	"github.com/aceagles/tui-toolkit/theme"
	"github.com/rivo/tview"
)

// Target is one address the tool acts on.
type Target struct {
	Addr   string // e.g. tcp://127.0.0.1:502; escaped
	Remote bool   // off this machine: drawn in RoleRemote
}

// Field is one labelled value. Label is escaped; Value may carry tview
// colour tags.
type Field struct {
	Label string
	Value string
}

// Conn is the link state shown after the fields.
type Conn int

const (
	ConnNone       Conn = iota // not shown
	ConnConnecting             // RoleWarn
	ConnUp                     // RoleOK
	ConnDown                   // RoleError, followed by ConnDetail
)

// Info is everything the header shows. Empty parts are left out.
type Info struct {
	Tool       string
	Targets    []Target
	Plane      string // e.g. modbus-tcp, redis, http
	Fields     []Field
	Conn       Conn
	ConnDetail string // shown after the state, e.g. the last error; escaped
	Pending    string // leader chord in flight, "" when idle
}

// Render returns one line of tview-tagged text in theme.Current colours.
func Render(h Info) string {
	p := theme.Current()
	muted := p.Tag(theme.RoleFgMuted)
	var parts []string
	if h.Tool != "" {
		parts = append(parts, "[::b]"+tview.Escape(h.Tool)+"[::-]")
	}
	for _, t := range h.Targets {
		addr := tview.Escape(t.Addr)
		if t.Remote {
			addr = p.Tag(theme.RoleRemote) + addr + "[-]"
		}
		parts = append(parts, addr)
	}
	if h.Plane != "" {
		parts = append(parts, muted+"plane[-] "+tview.Escape(h.Plane))
	}
	for _, f := range h.Fields {
		parts = append(parts, muted+tview.Escape(f.Label)+"[-] "+f.Value)
	}
	if state := connText(h.Conn, h.ConnDetail, p); state != "" {
		parts = append(parts, muted+"link[-] "+state)
	}
	if h.Pending != "" {
		parts = append(parts, fmt.Sprintf(" %s[::b]<space>%s[-::-]", p.Tag(theme.RoleAccent), tview.Escape(strings.TrimSpace(h.Pending))))
	}
	return strings.Join(parts, "  ")
}

func connText(c Conn, detail string, p theme.Palette) string {
	var role theme.Role
	var word string
	switch c {
	case ConnConnecting:
		role, word = theme.RoleWarn, "connecting"
	case ConnUp:
		role, word = theme.RoleOK, "connected"
	case ConnDown:
		role, word = theme.RoleError, "disconnected"
	default:
		return ""
	}
	if detail != "" {
		word += ": " + detail
	}
	return p.Tag(role) + tview.Escape(word) + "[-]"
}

// IsLoopback reports whether host names this machine: localhost, 127.0.0.0/8
// or ::1. It accepts a bare host, a bracketed IPv6 literal or host:port.
func IsLoopback(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
