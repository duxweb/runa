# Contributing to Runa

Runa is a multi-module Go repository. Contributions should keep the public API small, the dependency graph explicit, and capability packages installable on demand.

## Local development

Use the repository `go.work` file during local development:

```bash
git clone https://github.com/duxweb/runa.git
cd runa
go work sync
go test ./...
```

The project currently targets **Go 1.27rc1**. Use the same toolchain when running tests or generating release artifacts.

## Naming and structure

Follow the public naming guide in `docs/src/content/docs/contributing/naming.mdx`.

Core conventions:

- Capability packages expose `New()`, `Provider(...)`, and `Default()` when they enter the lifecycle.
- Driver interfaces are named `Driver` inside their package.
- Driver selection options are named `Use(name)`.
- Provider registration options use names such as `RegisterDriver(name, driver)` and `RegisterXxx(name, options...)`.
- Heavy external dependencies belong in driver submodules, not in the root framework module.

## Commands

Useful commands during development:

```bash
go run ./cmd/runa --help
go run ./cmd/runa dev --cmd serve
go run ./cmd/runa gen module billing
```

Run checks before opening a pull request:

```bash
go test ./...
go vet ./...
for mod in $(go list -m -f '{{.Dir}}'); do
  (cd "$mod" && go test ./... && go vet ./...)
done
```

Build documentation:

```bash
cd docs
pnpm install
pnpm build
```

## Pull requests

- Keep changes focused and explain the reason for the change.
- Add or update tests for behavior changes.
- Update Chinese and English documentation together when public behavior changes.
- Do not commit `_plans/`, local data, generated docs output, `node_modules`, or machine-specific paths.
- Do not introduce new root dependencies unless the feature requires them in the core API.

## Release work

Release steps are scripted in `scripts/release.sh`. Do not create tags, rewrite history, or push releases from a pull request.
