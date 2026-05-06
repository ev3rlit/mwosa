#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -n "${MWOSA_HELP_REPO_ROOT:-}" ]]; then
  repo_root="$MWOSA_HELP_REPO_ROOT"
elif repo_root="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  :
elif repo_root="$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null)"; then
  :
else
  echo "could not find mwosa repo root; run from the repo root or set MWOSA_HELP_REPO_ROOT" >&2
  exit 1
fi
mwosa_cmd="${MWOSA_HELP_COMMAND:-mwosa}"

targets=(
  "$repo_root/skills/mwosa/references/cli-command-help.md"
  "$repo_root/.agents/skills/mwosa/references/cli-command-help.md"
)

command_paths=()

tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT

run_mwosa() {
  "$mwosa_cmd" "$@"
}

extract_subcommands() {
  awk '
    /^Available Commands:/ {
      in_commands = 1
      next
    }
    in_commands && NF == 0 {
      exit
    }
    in_commands && $1 ~ /^[[:alnum:]_-]+$/ {
      print $1
    }
  '
}

collect_commands() {
  local prefix_string="${1:-}"
  local -a prefix=()
  if [[ -n "$prefix_string" ]]; then
    read -r -a prefix <<< "$prefix_string"
  fi

  local help_text
  if ((${#prefix[@]} > 0)); then
    help_text="$(run_mwosa "${prefix[@]}" --help)"
  else
    help_text="$(run_mwosa --help)"
  fi

  local child
  while IFS= read -r child; do
    [[ -z "$child" ]] && continue
    local child_path="${prefix_string:+$prefix_string }$child"
    command_paths+=("$child_path")
    collect_commands "$child_path"
  done < <(printf '%s\n' "$help_text" | extract_subcommands)
}

collect_commands

{
  printf '# mwosa CLI Command Help\n\n'
  printf 'Generated from `%s`. Use this when you need the complete installed or built CLI command surface instead of relying on source-code assumptions.\n\n' "$mwosa_cmd"
  printf '## Refresh Command\n\n'
  printf 'Run this from the repository root when the CLI changes:\n\n'
  printf '```bash\n'
  printf 'skills/mwosa/references/generate-cli-command-help.sh\n'
  printf '\n'
  printf '# Or use a freshly built binary:\n'
  printf 'MWOSA_HELP_COMMAND=./bin/mwosa skills/mwosa/references/generate-cli-command-help.sh\n'
  printf '\n'
  printf '# If running the global skill copy outside the repo:\n'
  printf 'MWOSA_HELP_REPO_ROOT=/path/to/mwosa skills/mwosa/references/generate-cli-command-help.sh\n'
  printf '```\n\n'
  printf '## Captured Help\n\n'
  printf '```text\n'

  run_mwosa version
  run_mwosa --help

  for command in "${command_paths[@]}"; do
    read -r -a args <<< "$command"
    printf '\n\n### mwosa %s --help\n' "$command"
    run_mwosa "${args[@]}" --help
  done

  printf '```\n'
} > "$tmp_file"

for target in "${targets[@]}"; do
  mkdir -p "$(dirname "$target")"
  cp "$tmp_file" "$target"
  printf 'wrote %s\n' "$target"
done
