# RULE.md

This file is the compact programming rulebook for mwosa. Keep it shorter than
the architecture documents. When a rule needs long explanation, link to the
relevant document instead of expanding this file.

## Core Principles

- Preserve the layer boundary. CLI code composes dependencies, service code uses
  role interfaces and repository interfaces, and provider implementation details
  do not leak into service code.
- Keep provider clients independent. A provider client under `providers/clients`
  is its own Go module with its own `go.mod` and tests.
- Prefer narrow changes. Do not introduce broad refactors when the request can
  be handled at the current boundary.
- Do not hide failures as empty success. Unsupported capability, missing route,
  remote error, storage error, and invalid input must return explicit errors
  with provider, group, operation, market, security type, symbol, or date
  context when those values exist.

## Error Handling

- Hand-written Go code must use `github.com/samber/oops` for error creation,
  wrapping, and joining.
- Do not use `fmt.Errorf`, `errors.New`, or `errors.Join` for new hand-written
  errors. Standard `errors.Is` and `errors.As` are allowed for error matching.
- Generated code, including `storage/ent`, is excluded from this rule because it
  is recreated by code generation.
- Use `oops.New` or `oops.Errorf` for a new error. Use `Wrap` or `Wrapf` when a
  lower-layer error is the cause. Use `oops.Join` when multiple cleanup errors
  must be preserved.
- Prefer reusable builders when a function repeats the same domain and context.
  The builder must be completed with `.New`, `.Errorf`, `.Wrap`, `.Wrapf`,
  `.Join`, `.Recover`, or `.Recoverf`.

```go
errb := oops.
	In("dailybar_repository").
	With(
		"market", query.Market,
		"security_type", query.SecurityType,
		"symbol", query.Symbol,
		"from", query.From,
		"to", query.To,
	)

client, err := r.database.Client(ctx)
if err != nil {
	return nil, errb.Wrap(err)
}
```

- Add context at each boundary instead of waiting until the CLI edge. The caller
  should add the request context it owns; the callee should add the domain
  context it owns.
- `With(...)` is structured context, not always a replacement for a human-facing
  message. If tests or CLI users must see fields such as `provider=datago`,
  `group=securitiesProductPrice`, `operation=getETFPriceInfo`, or `status=502`,
  keep those fields in the error message as well.

## Storage

- SQLite is the local canonical storage direction.
- Ent schemas live under `storage/schema`. Generated Ent code lives under
  `storage/ent` and should not be edited by hand.
- Runtime database access should be lazy. Creating a storage handle must not open
  SQLite; first actual use may open it, and command-level cleanup closes it.
- Repository concrete types stay unexported. Export constructors and return
  service-layer repository interfaces.
- Validate repository construction invariants in constructors, not repeatedly in
  every repository method.

## Providers

- Provider id and provider group are separate concepts. For datago, the provider
  id is `datago` and the first group is `securitiesProductPrice`.
- Do not encode provider group into a provider id with `-` or `/`.
- Provider adapters convert provider-native responses toward canonical models.
- Provider clients own endpoint paths, service keys, pagination, provider-native
  parsing, and remote error context.
- External API tests must use fake HTTP transports or `httptest`; unit tests must
  not depend on live public API calls.

## CLI

- Keep the CLI verb-first and consistent with `README.md`.
- Machine-readable output must remain predictable. JSON output should be
  structured for `jq`; human table output can be concise.
- stdout is for command results. stderr is for diagnostics, progress, and logs.

## Documentation

- Architecture contracts live under `docs/architectures`.
- Provider lists and provider-specific plans live under `docs/providers`.
- Do not reintroduce `docs/providers/provider-package-contract.md`; it was
  removed.
