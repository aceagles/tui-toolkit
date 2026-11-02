# tui-toolkit

Shared helpers for ad-hoc [tview](https://github.com/rivo/tview) workbench TUIs, so each tool doesn't re-implement the same vim-flavoured plumbing.

| Package | What it does |
| --- | --- |
| `tmuxnav` | `C-h/j/k/l` focus movement between registered panes, forwarding to `tmux select-pane` at the edge — vim-tmux-navigator semantics for a TUI. |
| `vimkeys` | Vim motions for tview widgets (`j/k`, `gg/G`, `C-d/C-u`) and a Space-leader chord dispatcher with groups, a which-key tray along the bottom of the screen, and a help table built from the same registry. |
| `extedit` | Suspend the app and open `$EDITOR` on a temp file; returns the saved contents. For JSON profiles and anything multi-line. |
| `theme` | tokyonight night and day palettes as semantic roles (`RoleWarn`, `RoleBorderFocus`, …). `Apply` sets `tview.Styles`, `Tag` gives colour tags, `Detect` follows tmux's light toggle, `Watch` switches live. |
| `confirm` | The confirmation dialogue for anything that changes a live system: labelled lines of exactly what will be sent; Cancel holds focus so Enter backs out; Esc cancels. |
| `header` | The one-line target header: tool, target(s), plane, fields, link state. A non-loopback target is drawn in the theme's Remote colour as plain text. `IsLoopback` decides. |
| `layout` | Pane sizing from what panes need: each declares `{Min, Pref, Max, Weight}` and a `Stack` re-solves every frame. Panes that cannot have their Min hide, rightmost first, with a `+N hidden` badge. `C-w H/J/K/L` resize mode, `Alt-hjkl` nudge, `C-w =` equalise, `C-w z` zoom. |

## Wiring

```go
app := tview.NewApplication()

nav := tmuxnav.New(app)
nav.Add(readout, 0, 0)
nav.Add(commands, 1, 0)
nav.Add(logPane, 0, 1)

leader := vimkeys.NewLeader(app, 0)
leader.MustRegister("r", "[R]efresh now", refresh)
leader.MustRegisterGroup("g", "groups")
leader.MustRegister("ga", "[a]dd group", addGroup)

app.SetRoot(vimkeys.NewTray(leader, root), true) // Space shows the tray
app.SetInputCapture(vimkeys.Chain(leader.InputCapture, nav.InputCapture))
logPane.SetInputCapture(vimkeys.Motions())
```

Put the leader first in the chain so a pending chord owns every key, including `C-h`. The tray has no timeout; without a tray the leader falls back to an 800ms chord timeout. `vimkeys.CtrlCAsEscape` wraps the app-level capture so `C-c` acts as `Esc` (cancel the edit, close the dialogue, abandon the chord) and never quits; tview otherwise stops the app on `C-c`. Give the tool an explicit quit key. A `kill -INT` still ends the process.

```go
app.SetInputCapture(vimkeys.CtrlCAsEscape(vimkeys.Chain(leader.InputCapture, nav.InputCapture)))
```

Tools with modals set `leader.SkipWhen = func() bool { return modalOpen() || vimkeys.FocusWantsSpace(app) }`.

## Layout

```go
body := layout.NewStack(layout.Horizontal)
body.Add(list, layout.Spec{Min: 24, Pref: 30, Priority: 1})
body.Add(table, layout.Spec{Min: 20}).SetSpecFunc(tableSpec) // Pref from content, every frame
area := layout.NewStack(layout.Vertical)
area.Add(body, layout.Spec{Min: 5, Weight: 1, Priority: 1})
area.Add(logPane, layout.Spec{Min: 5, Pref: 9})

lay := layout.New(app, area) // area is what a zoomed pane fills
lay.SkipWhen = func() bool { _, chord := leader.Pending(); return chord || modalOpen() || layout.TextFocus(app) }
lay.OnState = redrawHeaderAndNav // header shows lay.Status(); register only lay.Visible panes with tmuxnav
leader.MustRegister("z", "[z]oom pane", lay.ToggleZoom)
app.SetInputCapture(vimkeys.Chain(lay.InputCapture, leader.InputCapture, nav.InputCapture))
```

Sizing is greedy: every pane gets its Min, then its Pref in order, then what is left by Weight up to Max (0 = unbounded). Space nobody wants stays empty; nothing stretches without a Weight. When the Mins do not fit, the lowest-Priority, rightmost panes hide and come back in reverse order as the terminal widens.

`C-w H/J/K/L` moves a border 5 cells and stays in resize mode, so `C-w L L L` moves 15; any other key leaves the mode and is then handled as normal, and Esc leaves silently. The key names the direction the border moves, as in tmux: the focused pane's right (bottom) border if it has a neighbour there, else its left (top). Resizes are clamped to Min and Max and re-clamped when the terminal shrinks. `C-w z` (or `C-w o`) zooms the focused pane over the area; again or Esc restores it and the focus. Put `lay.InputCapture` first in the chain so the key that ends resize mode reaches everything after it.

## Theme

```go
theme.EnableTrueColor()
theme.Apply(theme.Detect()) // before building widgets
defer theme.Watch(app, 0, func(p theme.Palette) { theme.Restyle(pages, p); redraw() })()
```

Colour by role, never by name: `p.Tag(theme.RoleWarn)` in tagged text, `p.Color(theme.RoleBorderFocus)` on widgets. `Detect` order: `TUI_THEME=night|day`, then the tmux global option `@dotbar-light` (set means day; change `theme.TmuxLightOption` for another toggle), then night. `Watch` polls every 2s, re-applies and calls you on the event loop; `Restyle` remaps existing widgets, and your callback re-renders any tagged text.

Truecolor: tcell's `tmux-256color` entry has no RGB capability, so under tmux it rounds the palette to the nearest of 256 colours unless `COLORTERM=truecolor`. `EnableTrueColor` sets that for the process when tmux reports `RGB` in `#{client_termfeatures}` (tmux `terminal-overrides ",*256col*:Tc"` or `terminal-features ",*:RGB"`). It leaves an existing `COLORTERM` or `TCELL_TRUECOLOR` alone. Outside tmux, or with no client attached, export `COLORTERM=truecolor` yourself if the terminal supports it.

## tmux setup

`tmuxnav` only receives `C-h/j/k/l` if tmux passes them through. Two requirements:

1. Name the binary `<something>tui` (`lc300tui`, `dfrprobetui`).
2. Widen vim-tmux-navigator's `is_vim` check to match `*tui` processes — after the tpm run line in `~/.tmux.conf`:

```tmux
is_vim_or_tui="ps -o state= -o comm= -t '#{pane_tty}' \
    | grep -iqE '^[^TXZ ]+ +(\\S+\\/)?g?(view|l?n?vim?x?|fzf|\\S+tui)(diff)?$'"
bind-key -n 'C-h' if-shell "$is_vim_or_tui" 'send-keys C-h' 'select-pane -L'
bind-key -n 'C-j' if-shell "$is_vim_or_tui" 'send-keys C-j' 'select-pane -D'
bind-key -n 'C-k' if-shell "$is_vim_or_tui" 'send-keys C-k' 'select-pane -U'
bind-key -n 'C-l' if-shell "$is_vim_or_tui" 'send-keys C-l' 'select-pane -R'
```

Without the tmux conf, bind `Tab`/`Shift-Tab` as the pane-cycle fallback in the tool.
