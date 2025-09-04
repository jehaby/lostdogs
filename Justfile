set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

GOOSE := `if command -v goose >/dev/null 2>&1; then printf goose; else printf "go run github.com/pressly/goose/v3/cmd/goose@v3.25.0"; fi`

@default:
    just --list

migrate-up:
    {{GOOSE}} up

migrate-down:
    {{GOOSE}} down

migrate-status:
    {{GOOSE}} status

migrate-create name:
    {{GOOSE}} create {{name}} sql
