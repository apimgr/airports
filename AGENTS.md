# airports — Project Memory

This project uses the AI.md specification system.

- **AI.md** — implementation spec (HOW). Source of truth. Read-only; never modify.
- **IDEA.md** — project plan (WHAT). Business logic, features, project variables.
- **TODO.AI.md** / **TODO.md** — task tracking (AI-owned / human-owned).
- **PLAN.AI.md** / **PLAN.md** — implementation plans (AI-owned / human-owned).

## Before any task

1. Identify which AI.md PART(s) apply to the current task — read only those, not the whole file.
2. Check TODO.AI.md and TODO.md for pending work.
3. Never guess — if the spec doesn't cover a case, ask.

## Rule cheatsheets

`.claude/rules/*.md` hold condensed, regenerated-from-AI.md summaries per topic (not committed —
regenerate locally after cloning if missing). See AI.md PART 0 → "Rule Files to Create/Update"
for the full file → PART mapping.

## Project identity

- name: airports · org: apimgr · binary: airports · client: airports-cli
- license: MIT
