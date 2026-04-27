# AGENTS.md

This repository uses layered project instructions.

## Required Reading

- Read `RULE.md` before changing code.
- Read the relevant architecture document before changing a boundary:
  `docs/architectures/provider/README.md`,
  `docs/architectures/layers/README.md`, or
  `docs/architectures/tech-stack/README.md`.
- Read the relevant provider document before changing provider behavior:
  `docs/providers/README.md` and the provider-specific README.

## Working Rules

- Follow `RULE.md` as the primary programming guide.
- Keep commits narrow and topic-based.
- Run `gofmt` after Go edits.
- Run `go test ./...` from the repository root after code changes.
- If a provider client module changes, run its module-local `go test ./...` and
  `go mod verify` as well.

## Generated Code

- Do not edit `storage/ent` by hand.
- Change Ent schema under `storage/schema`, then regenerate generated code.
