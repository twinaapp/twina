# Twina

A tiny cross-platform dual-panel file manager.

## Run

```sh
wails3 dev      # hot-reload dev mode
wails3 build    # produces bin/Twina
go test .       # backend tests (TWINA_TRASH_TEST=1 also tests the real Trash)
```

## Keys

| Key | Action |
| --- | --- |
| Tab | Switch panel |
| ↑ ↓ PgUp PgDn Home End | Move cursor |
| Enter / double-click | Open folder, or open file in default app |
| Backspace, ⌘↑ | Parent folder |
| Space, Insert, Shift+↑↓ | Toggle selection |
| ⌘A / ⌘D / Esc | Select all / clear selection |
| ⌘-click, Shift-click | Toggle / range select |
| F2, Shift+F6 | Rename |
| F3 | View file (text viewer) |
| F4 | Open with default app |
| F5 / F6 | Copy / move to the other panel (destination editable; asks before replacing; progress + Esc to cancel) |
| F7 | New folder |
| F8, Delete, ⌘⌫ | Move to Trash |
| Shift+F8, Shift+Delete, ⌘⌥⌫ | Delete permanently |
| ⌘T / ⌘W | New tab / close tab (middle-click also closes) |
| Ctrl+Tab, ⌘⇧] / ⌘⇧[ | Next / previous tab |
| ⌘1…⌘9 | Go to tab (⌘9 = last) |
| ⌘R | Refresh both panels |
| ⌘L / double-click path | Edit path (supports `~`) |
| ⌘U | Swap panels |
| ⌘O | Show current folder in the other panel |
| ⌘← / ⌘→ | Open current folder in left / right panel |
| ⌘. | Toggle hidden files |
| Typing letters | Jump to matching name |

On Mac laptops, hold `fn` for F-keys, or use the button bar at the bottom.

## Layout

- `fileservice.go` — Go service bound to the frontend: list, rename, mkdir, trash, open, read.
- `transfer.go` — copy/move with overwrite/merge, progress events and cancellation (via the call's context).
- `updates.go` — update checks against GitHub Releases (release builds only; `version.go` stays `0.0.0` locally). Installs in place on macOS/Windows, links to the release on Linux.
- `trash_*.go` — system Trash per platform (NSWorkspace on macOS, freedesktop on Linux, Recycle Bin on Windows).
- `frontend/src/lib/state.svelte.ts` — per-tab panel state (sorting, selection, cursor) and per-side tabs, using runes.
- `frontend/src/lib/Icon.svelte` — file-type icons.
- `frontend/src/lib/Panel.svelte` — a single file panel.
- `frontend/src/App.svelte` — two panels, keyboard handling, file operations, F-key bar.
