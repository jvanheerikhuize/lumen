# Contributing

Thanks for helping out! Bug reports, ideas and pull requests are all welcome.

## Getting set up

You need **Go 1.24+**, **Node.js 22.18+**, and **ffmpeg** (see the [README](README.md#requirements)).

```bash
npm install
npm run build
go build -o bin/lumen ./cmd/lumen
./bin/lumen
```

For frontend development with hot module replacement (proxying `/api/*` to the Go backend on port `3000`):

```bash
npm run dev
```

## Before opening a pull request

```bash
go vet ./...
npm run typecheck
npm test
python3 tools/spec.py check --strict
npm run build
go build -o bin/lumen ./cmd/lumen
```

CI runs the same checks on every pull request.

## Guidelines

- Keep pull requests small and focused on one change.
- Write commit messages in the [Conventional Commits](https://www.conventionalcommits.org/) style, e.g. `feat(live): add channel favourites` or `fix(player): resume after seeking`.
- Add or update table-driven Go tests in `internal/*/*_test.go` (or frontend tests in `tests/`) for logic you change. UI changes are easier to review with a screenshot.
- When changing domain behavior or architectural facts, update the corresponding epistemic records in `records/` and run `npm run spec:check` and `npm run spec:render`.
- Only free, openly available services are built in. Anything that needs a paid account belongs behind an optional setting.
- Please don't add indexers or sources whose main purpose is piracy.

## Reporting bugs

Open an issue with what you did, what you expected and what happened. Include your OS, Go version, and browser. For security issues, see [SECURITY.md](SECURITY.md) instead.

By contributing you agree that your work is released under the [MIT License](LICENSE).
