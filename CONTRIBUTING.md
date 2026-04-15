# Contributing

Thanks for your interest in contributing.

## Development Setup

1. Fork the repository and create a feature branch from `main`.
2. Copy `.env.example` to `.env` and fill required values.
3. Start dependencies locally (optional): `docker compose up -d`.
4. Run the API: `make run`.

## Quality Checks

Before opening a pull request, run:

- `make fmt`
- `make lint`
- `make test`
- `make build`

If you update public API handlers or models, regenerate Swagger docs:

- `make swagger`

## Pull Request Guidelines

- Keep changes focused and small when possible.
- Include tests for new behavior or bug fixes.
- Update documentation when behavior or configuration changes.
- Fill the pull request template completely.

## Commit Guidance

- Use clear commit messages that explain the intent.
- Avoid mixing refactors and behavior changes in the same commit unless necessary.
