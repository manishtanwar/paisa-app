---
name: dev-workflow
description: Git branch and development workflow for this repo (paisa fork) — creating a feature branch off main-manish, preparing it to merge by updating MY_CHANGELOG.md, and squash-merging it into main-manish. Use when the user asks to start/create a new feature branch, wants to mark work as merge-ready, or wants to merge/squash a branch into main-manish.
---

# Dev Workflow

Integration branch: `main-manish`. Release branch: `release-manish`.

## 1. Create a feature branch

Branch off `main-manish`:

```bash
git checkout main-manish
git pull
git checkout -b u/manish/<feature-name>
```

`<feature-name>` is short kebab-case (e.g. `u/manish/stock-dashboard`, `u/manish/bug/fix-import`).

## 2. Make it merge-ready

Before merging, add a bullet describing the feature to `MY_CHANGELOG.md` (repo root), matching the existing list style — one line per change, terse, past tense.

## 3. Squash merge into main-manish

Local squash merge (no PR):

```bash
git checkout main-manish
git pull
git merge --squash u/manish/<feature-name>
git commit -m "<detailed description of the feature>"
git push
```

The commit message must be the actual detail of the feature (what was added/changed/fixed), not a generic message like "squash merge" or "merge branch". It should read like the `MY_CHANGELOG.md` entry, expanded if useful.

Don't do cleanup of feature branch (let that be there).
