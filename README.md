# CCManager

Gamified macro manager for Claude Code sessions. Inspired by StarCraft 2 mechanics, CCManager brings APM tracking, control groups, streak multipliers, and a Pomodoro timer to your coding workflow.

## Features

- Real-time session dashboard with state detection (active, idle, thinking, urgent)
- Control groups (1-9, 0 hotkeys) for quick session switching
- Gamification: APM tracking, streak multipliers, scoring system
- Integrated Pomodoro timer with work/break cycles
- SQLite persistence for statistics and session data
- Preview pane with live session output
- Workspace and worktree support (git, jj)

## Prerequisites

- **Go 1.25+**
- **tmux** (required for session monitoring)
- macOS or Linux

## Installation

```bash
# Clone
git clone https://github.com/valentindosimont/ccmanager
cd ccmanager

# Build
make build
# or: go build -o bin/ccmanager ./cmd/ccmanager

# Run
./bin/ccmanager
```

## Configuration

Config file: `~/.config/ccmanager/config.yaml`

See `config.example.yaml` for all options. Key settings:

```yaml
editor: cursor

pomodoro:
  work_minutes: 25
  short_break_minutes: 5

streak:
  timeout_seconds: 30
  multiplier_cap: 10.0

scoring:
  points_per_action: 10
  points_pomodoro_complete: 1000
```

## Keybindings

### Navigation
| Key | Action |
|-----|--------|
| `↑/k`, `↓/j` | Move selection |
| `Enter` | Focus session (switch tmux) |
| `Tab` | Cycle sessions |

### Sessions
| Key | Action |
|-----|--------|
| `n` | Create new session |
| `dd` | Delete selected session |
| `[` | Toggle session list |
| `e` | Open editor in session dir |

### Preview
| Key | Action |
|-----|--------|
| `Ctrl+U` | Scroll preview up |
| `Ctrl+D` | Scroll preview down |
| `G` | Jump to bottom |

### Prompt
| Key | Action |
|-----|--------|
| `i`, `/` | Enter prompt mode |
| `Enter` | Send command to session |
| `Esc` | Exit prompt mode |
| `x` | Cancel Claude task |
| `c` | Interrupt Claude (Ctrl+C) |
| `Shift+Tab` | Cycle Claude mode |
| `D` | Show activity overlay |

### Control Groups
| Key | Action |
|-----|--------|
| `1-9`, `0` | Tap: cycle, Double-tap: focus |
| `g + 1-9` | Assign selected to group |

### Pomodoro
| Key | Action |
|-----|--------|
| `p` | Start/pause pomodoro |
| `P` | Stop pomodoro |
| `s` | Show statistics |

### General
| Key | Action |
|-----|--------|
| `?` | Toggle help |
| `q` | Quit |

## License

MIT
