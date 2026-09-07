# tui-toolkit

Shared helpers for ad-hoc [tview](https://github.com/rivo/tview) workbench TUIs, so each tool doesn't re-implement the same vim-flavoured plumbing.

| Package | What it does |
| --- | --- |
| `tmuxnav` | `C-h/j/k/l` focus movement between registered panes, forwarding to `tmux select-pane` at the edge — vim-tmux-navigator semantics for a TUI. |
| `vimkeys` | Vim motions for tview widgets (`j/k`, `gg/G`, `C-d/C-u`) and a Space-leader chord dispatcher with a help table. |
| `extedit` | Suspend the app and open `$EDITOR` on a temp file; returns the saved contents. For JSON profiles and anything multi-line. |

## Wiring

```go
app := tview.NewApplication()

nav := tmuxnav.New(app)
nav.Add(readout, 0, 0)
nav.Add(commands, 1, 0)
nav.Add(logPane, 0, 1)

leader := vimkeys.NewLeader(app, 0)
leader.MustRegister("r", "[R]efresh now", refresh)

app.SetInputCapture(vimkeys.Chain(nav.InputCapture, leader.InputCapture))
logPane.SetInputCapture(vimkeys.Motions())
```

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
