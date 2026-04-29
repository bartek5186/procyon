# Using this repo with Cursor

This project includes two **Cursor project rules** that apply automatically when you work here.

## In this repository

1. Open the folder in Cursor.
2. Both rules in `.cursor/rules/` are committed with `alwaysApply: true` — no extra setup needed:
   - `karpathy-guidelines` — behavioral principles (think before coding, simplicity, surgical changes, goal-driven execution)
   - `architecture-api-guidelines` — project architecture and patterns (controller/service/store layers, code patterns, auth, dev commands)
3. Confirm them under **Settings → Rules** (or the project rules UI).

## Use the same guidelines in another project

**Cursor:** Copy both `.cursor/rules/*.mdc` files into that project's `.cursor/rules/` directory.

**Claude Code:** Copy or symlink both skill files into your personal `~/.claude/skills/` directory:
- [`skills/karpathy-guidelines/SKILL.md`](skills/karpathy-guidelines/SKILL.md)
- [`skills/architecture-api-guidelines/SKILL.md`](skills/architecture-api-guidelines/SKILL.md)

**Other tools:** Merge content from both skill files into the project's root instruction file.

## For contributors

When you change the guidelines, keep the rule files and their corresponding skill files in sync:

| Rule | Skill |
|------|-------|
| `.cursor/rules/karpathy-guidelines.mdc` | `skills/karpathy-guidelines/SKILL.md` |
| `.cursor/rules/architecture-api-guidelines.mdc` | `skills/architecture-api-guidelines/SKILL.md` |

`CLAUDE.md` references both skill files and stays as a lightweight index.
