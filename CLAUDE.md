# Project conventions

## Issue tracking

Issues are managed in GitHub. Always use `gh` to interact with them — never look for local markdown files in an `issues/` directory.

| Task | Command |
|------|---------|
| List open issues | `gh issue list` |
| View an issue | `gh issue view <number>` |
| Search issues | `gh issue list --search "<query>"` |
| List closed issues | `gh issue list --state closed` |

## PRD

The product requirements document lives at `PRD.md` in the repo root.
