# CCManager - Claude Code Session Manager

Go CLI application with Bubble Tea TUI for managing Claude Code sessions with gamification features.

## Development Commands

```bash
make build    # Build binary to bin/ccmanager
make test     # Run all tests
make lint     # Run golangci-lint
make deps     # Run go mod tidy
make dev      # Build and run
make run-debug # Run with CCMANAGER_DEBUG=1
```

## Code Style

- Follow Go conventions and existing patterns in the codebase
- Use standard library where possible
- Keep functions focused and small
- Error handling: return errors, don't panic

## Project Structure

```
cmd/ccmanager/     # Main entry point
internal/
  app/             # Application orchestration
  claude/          # Claude Code process detection
  config/          # Configuration management
  daemon/          # Background monitoring
  game/            # Gamification (streaks, pomodoro, control groups)
  store/           # SQLite persistence
  tmux/            # Tmux integration
  tui/             # Bubble Tea UI components
  usage/           # Usage tracking and parsing
  workspace/       # Workspace/project detection (git, jj)
```

## Before Committing

1. Run tests: `make test`
2. Run linter: `make lint`
3. Ensure build passes: `make build`

## Testing

- Tests are in `*_test.go` files alongside source
- Run specific package: `go test -v ./internal/game/...`
- Run with coverage: `go test -coverprofile=coverage.out ./...`
