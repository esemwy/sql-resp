---
name: prd-to-issues
description: Break a PRD into independently-workable issues and create them as real GitLab issues using glab. Use when the user wants to turn a PRD into a list of concrete tasks.
---

# PRD to GitLab Issues

Break a PRD into independently-grabbable issues as vertical slices (tracer bullets) and create them directly in GitLab using `glab`.

## Process

### 1. Locate the PRD

Read `PRD.md` from the repo root. If it doesn't exist, ask the user for the path.

### 2. Verify GitLab project access

Run `glab repo view` to confirm the current repo has a GitLab remote and `glab` can reach it.

If the command fails because no GitLab remote exists, offer to create the project with `glab repo create`, then proceed once it's set up.

### 3. Explore the codebase (optional)

If you have not already explored the codebase, do so to understand the current state of the code.

### 4. Draft vertical slices

Break the PRD into **tracer bullet** issues. Each issue is a thin vertical slice that cuts through ALL integration layers end-to-end, NOT a horizontal slice of one layer.

Slices may be 'HITL' or 'AFK'. HITL slices require human interaction, such as an architectural decision or a design review. AFK slices can be implemented and merged without human interaction. Prefer AFK over HITL where possible.

<vertical-slice-rules>
- Each slice delivers a narrow but COMPLETE path through every layer (schema, API, UI, tests)
- A completed slice is demoable or verifiable on its own
- Prefer many thin slices over few thick ones
</vertical-slice-rules>

### 5. Quiz the user

Present the proposed breakdown as a numbered list. For each slice, show:

- **Title**: short descriptive name
- **Type**: HITL / AFK
- **Blocked by**: which other slices (if any) must complete first
- **User stories covered**: which user stories from the PRD this addresses

Ask the user:

- Does the granularity feel right? (too coarse / too fine)
- Are the dependency relationships correct?
- Should any slices be merged or split further?
- Are the correct slices marked as HITL and AFK?

Iterate until the user approves the breakdown.

### 6. Create GitLab issues

Create issues in dependency order (blockers first) so you can reference real GitLab issue numbers in later issues' "Blocked by" sections.

For each issue, run:

```
glab issue create --title "<title>" --description "<body>" --label "<AFK or HITL>"
```

Capture the issue URL/number from each `glab` command's output before moving to the next issue so you can reference it in dependent issues.

Format the `--description` value using this template:

<issue-template>
## What to build

A concise description of this vertical slice. Describe the end-to-end behavior, not layer-by-layer implementation.

## Acceptance criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Blocked by

- #NNN <title> (if any — use the real GitLab issue number captured from glab output)

Or "None — can start immediately" if no blockers.

## User stories addressed

Reference by number from PRD.md:

- User story 3
- User story 7
</issue-template>

After all issues are created, print a summary table of the created issues with their GitLab URLs.
