# radioman

An Internet radio client in Go featuring:
- A **Terminal User Interface (TUI)** built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- A **CLI & detached background player** (`-c`) synced with the TUI
- A **RadioBrowser API client library** (`github.com/erbekin/radioman`) with DNS SRV server discovery, failover, and ICY metadata streaming

## Prerequisites

- **Go** 1.21+ (built with Go 1.27)
- At least one supported audio player installed (`PATH` or standard OS install location):
  - `mpv`
  - `ffplay` (FFmpeg)
  - `vlc`

## Installation

```sh
git clone git@github.com:erbekin/radioman.git
cd radioman
go build -o radioman ./cmd/radioman
```

Or install directly:

```sh
go install github.com/erbekin/radioman/cmd/radioman@latest
```

## TUI Usage

Launch the interactive TUI:

```sh
radioman
```

### Startup Flags

| Flag | Description |
| :--- | :--- |
| `-search <name>` | Initial station name filter (positional arg also works) |
| `-country <name\|ISO-2>` | Initial country or 2-letter country code filter |
| `-tag <genre>` | Initial tag/genre filter (e.g. `jazz`, `rock`, `news`) |
| `-favorites` | Start directly in the Favorites tab |
| `-visualizer` | Enable the audio visualizer on startup |
| `-limit <n>` | Maximum number of stations to fetch (default `50`) |
| `-player <mpv\|ffplay\|vlc>` | Preferred audio player backend |
| `-c` | Run in CLI mode (non-interactive) |

### TUI Keybindings

| Key | Action |
| :--- | :--- |
| `↑`/`↓` or `j`/`k`, `PgUp`/`PgDn`, `g`/`G` | Navigate station list |
| `Enter` or `p` | Play / Stop selected station |
| `x` or `Space` | Stop playback |
| `f` | Toggle favorite (`★`) on selected station |
| `e` | Edit short name (`sname`) of selected favorite |
| `b` | Toggle **continue playing on exit** (`ON`/`OFF`) |
| `v` / `m` | Toggle visualizer / Cycle visualizer mode (`Spectrum Bars`, `Mirror EQ`, `Waveform`) |
| `/` or `s` | Open/focus search filters (`Tab` to switch fields, `Enter` to search, `Esc` to cancel) |
| `1` / `2` / `3` / `Tab` | Switch between Top Stations, Search, and Favorites |
| `r` | Refresh current view |
| `q` or `Ctrl+C` | Quit |

## CLI & Background Player (`-c`)

`radioman -c <command>` controls playback in the background and shares state with the TUI.

### Commands

| Command | Alias | Description |
| :--- | :--- | :--- |
| `radioman -c favs list` | `fl` | List favorite stations with numeric `<idx>` and `<sname>` |
| `radioman -c favs play <idx\|sname>` | `fp <idx\|sname>` | Play a favorite station in the background |
| `radioman -c favs rename <idx\|sname> <new>` | — | Rename a favorite's short name (`sname`) |
| `radioman -c player` | — | Print player backend, status, and current station |
| `radioman -c player play` | `pp` | Resume/play the current station (errors if none is active) |
| `radioman -c player stop` | `ps` | Stop the background player while remembering the current station |
| `radioman -c player current` | `pc` | Print detailed info for the current station |
| `radioman settings` (or `-c settings`) | — | Show current settings |
| `radioman settings continue_on_exit <on\|off>` | — | Enable or disable continuing playback when TUI exits |

### Favorites, `<idx>`, and `<sname>`

- Every favorited station receives a persistent unique numeric `<idx>` and a unique short name (`<sname>`, characters `a-zA-Z0-9-_.`).
- Default `<sname>` values are auto-generated from the station title (e.g. `"KralTürk"` → `KralTurk`, `"Christmas FM Non-Stop!"` → `Christmas-FM-Non-Stop`, `"DFM Дискач 90-х"` → `DFM-90-x`).
- You can rename any favorite's `sname` with `e` in the TUI or `radioman -c favs rename <idx|sname> <new-sname>`.

### TUI & CLI Sync and Continue-on-Exit

- Playback state is persisted in `player_state.json`, so starting/stopping in the CLI immediately reflects in the TUI and vice versa.
- `continue_on_exit` is **`true` by default**: closing the TUI leaves TUI-started playback running in the background (use `radioman -c ps` or `b` in the TUI to change this behavior).

## Configuration Files

Stored in your OS user config directory (`<UserConfigDir>/radioman/`, e.g. `%AppData%\radioman\` on Windows or `~/.config/radioman/` on Linux):

- `favorites.json` — saved stations with `id` and `sname`
- `player_state.json` — active background player PID, backend, and current station
- `settings.json` — persistent settings (`continue_on_exit`)

## Go Library Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	radiobrowser "github.com/erbekin/radioman"
)

func main() {
	client, err := radiobrowser.NewClient(nil)
	if err != nil {
		log.Fatal(err)
	}

	stations, err := client.TopStations(context.Background(), 10)
	if err != nil {
		log.Fatal(err)
	}

	for _, st := range stations {
		fmt.Printf("%s (%s %dkbps)\n", st.Name, st.Codec, st.Bitrate)
	}
}
```
