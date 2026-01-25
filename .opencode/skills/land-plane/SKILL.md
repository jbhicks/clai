---
name: land-plane
description: Clean up branch and push changes with proper documentation
license: MIT
compatibility: opencode
metadata:
  audience: developers
  workflow: git
---

## What I do
- Update all documentation based on changes in the current branch
- Add all changed files to git staging
- Create a commit with a relevant message based on the changes made
- Push the commit to the remote repository

## When to use me
Use this when you've completed work on a feature/bug fix and want to "land" the changes cleanly.

I will:
1. Analyze what files have changed and their content
2. Update relevant documentation (README, API docs, etc.) based on the changes
3. Stage all modified and new files
4. Generate an appropriate commit message following the project's commit message style
5. Push the changes to the remote repository

## Implementation Details

### Documentation Updates
- Check for new functions/classes and add to API documentation
- Update README.md if there are breaking changes or new features
- Update CHANGELOG.md if it exists
- Ensure all public APIs have proper documentation

### Git Workflow
- Run `git status` and `git diff` to understand changes
- Stage all changes with `git add .`
- Follow existing commit message patterns in the repo
- Push to the correct remote branch

### Quality Checks
- Ensure no sensitive data is being committed
- Check that all tests pass before committing
- Verify documentation is accurate and up-to-date