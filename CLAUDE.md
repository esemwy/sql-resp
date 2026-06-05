# Project conventions

## Issue tracking

Issues are managed in GitLab. Always use `glab` to interact with them — never look for local markdown files in an `issues/` directory.

| Task | Command |
|------|---------|
| List open issues | `glab issue list` |
| View an issue | `glab issue view <number>` |
| Search issues | `glab issue list --search "<query>"` |
| List closed issues | `glab issue list --state closed` |

## PRD

The product requirements document lives at `PRD.md` in the repo root.
