# Repository Guidelines

## Project Structure & Modules
- `main.go`: VK wall scanner and Telegram notifier (entrypoint and logic).
- `go.mod`/`go.sum`: module metadata (`github.com/jehaby/lostdogs`).
- `.envrc`: local environment variables for `direnv` (tokens, chat IDs). Avoid real secrets in VCS.
 
## Build, Test, and Development
- Run locally:
  
  ```sh
  VK_TOKEN=... TG_TOKEN=... TG_CHAT=-1001234567890 go run .
  ```
- Build binary:
  
  ```sh
  go build -o bin/lostdogs .
  ```
- After every code change:
  
  ```sh
  go fmt ./...
  ```
- Format and vet:
  
  ```sh
  go fmt ./...
  go vet ./...
  ```
- Tests (when added):
  
  ```sh
  go test ./...
  ```

## Coding Style & Naming
- Use standard Go formatting (`gofmt` via `go fmt`). Tabs for indentation; 100–120 col soft limit.
- Package names: short, lower-case; identifiers: `CamelCase` for exported, `lowerCamel` for internal.
- Files: use descriptive snake_case (e.g., `telegram_client.go`, `wall_scan.go`).
- Errors: wrap with context (`fmt.Errorf("...: %w", err)`). Log user-facing messages concisely.

## Testing Guidelines
- Framework: github.com/stretchr/testify with table-driven tests. Name files `*_test.go`.
- Focus: parsing/normalization utilities (`normalize`, `truncate`), attachment handling, and rate-limiting logic.
- Add hermetic tests; mock HTTP for Telegram/VK calls. Run `go test -race ./...` for data races.

## Commit & Pull Requests
- Commits: imperative, scoped messages. Example: `scan: de-duplicate posts by owner/id`.
- PRs must include:
  - Summary of change and rationale
  - Linked issue (e.g., `Fixes #12`)
  - How to test (env vars, example command)
  - Logs or screenshots, if behavior-facing

## Security & Configuration
- Required env vars: `VK_TOKEN`, `TG_TOKEN` (bot), `TG_CHAT` (chat id).
- Prefer `direnv` for local use (`.envrc`); do not commit production secrets. Rotate any accidentally exposed tokens.
- Network timeouts and polling: adjust in `main.go` (`ticker`, HTTP `Timeout`).
- Group sources: edit `groups` in `main.go`. Consider moving to config file for deployments.

## Notes
- Go version: use the version from `go.mod` (or latest stable). Vendor or pin dependencies if reproducibility matters.

## Dependencies
use this dependencies for corresponding functionality (update to latest versions)
  
	// github.com/caarlos0/env/v11 v11.3.1
	// github.com/go-telegram/bot v1.15.0
	// github.com/goccy/go-yaml v1.17.1
	// github.com/mattn/go-sqlite3 v1.14.28

### JSON

- Use: import `encoding/json/v2` in all code.
- Toolchain: requires `GOEXPERIMENT=jsonv2` or a toolchain with v2 enabled.
- Basics: use `json.Marshal`/`json.Unmarshal` as usual.
- Strict external input: `(json.UnmarshalOptions{RejectUnknownFields: true}).Unmarshal(data, &v)`.
- Defaults: allow unknown fields otherwise; avoid v1-only APIs (e.g., `Decoder.UseNumber`).
- Do: use option structs for strictness; keep a single JSON package per package.
- Don’t: import `encoding/json`; mix v1 and v2; silently discard unknown fields where correctness matters.
