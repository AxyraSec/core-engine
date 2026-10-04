# GitHub Settings Checklist

Manual settings for the `AxyraSec` organization and the `core-engine` repository.
These are not stored in the repo, so check them by hand after any change.

## Plan limits (Free organization)

On the GitHub Free plan for organizations:

- **Rulesets and branch protection work on public repositories only.** On a private
  repository, the rules can be created but are **not enforced**. To enforce them on a
  private repo, the organization needs GitHub Team (or higher).
- **Secret scanning, push protection and CodeQL code scanning** are free on public
  repositories. On private repositories they need the paid GitHub Secret Protection and
  GitHub Code Security add-ons.
- **Dependabot alerts and security updates** are free on all repositories.

If the repo stays private on the Free plan: keep the settings below anyway (they apply as
soon as the repo goes public or the plan changes), and rely on local checks
(`make lint test security`, pre-commit hooks) and discipline until then.

## Organization

Settings → Authentication security

- [ ] Require two-factor authentication for everyone in the organization.

Settings → Member privileges

- [ ] Base permissions: **Read** (or No permission).
- [ ] Disable repository creation by members if not needed.
- [ ] Disable forking of private repositories.

Settings → Actions → General

- [ ] Workflow permissions: **Read repository contents and packages permissions** (read-only by default).
- [ ] Do **not** allow GitHub Actions to create or approve pull requests.
- [ ] Fork pull request workflows: **Require approval for all external contributors**.

## Repository: General

Settings → General → Pull Requests

- [ ] Allow squash merging: **on** (default message: pull request title).
- [ ] Allow merge commits: **off**.
- [ ] Allow rebase merging: **off**.
- [ ] Automatically delete head branches: **on**.
- [ ] Always suggest updating pull request branches: **on**.

## Repository: ruleset on `main`

Settings → Rules → Rulesets → New branch ruleset

- [ ] Name: `main`. Enforcement status: **Active**.
- [ ] Target branches: **Default branch**.
- [ ] Bypass list: empty (no bypass, not even admins).
- [ ] Restrict deletions.
- [ ] Block force pushes.
- [ ] Require linear history.
- [ ] Require signed commits.
- [ ] Require a pull request before merging:
  - [ ] Required approvals: 1 (set to 0 while you are the only maintainer, otherwise you cannot merge).
  - [ ] Dismiss stale approvals when new commits are pushed.
  - [ ] Require review from Code Owners.
  - [ ] Require conversation resolution before merging.
  - [ ] Allowed merge methods: **Squash** only.
- [ ] Require status checks to pass:
  - [ ] Require branches to be up to date before merging.
  - [ ] Add the CI and security jobs once the workflows exist and have run at least once
        (they only appear in the list after a first run).

## Repository: ruleset on tags

Settings → Rules → Rulesets → New tag ruleset

- [ ] Name: `release-tags`. Enforcement status: **Active**.
- [ ] Target tags: `v*`.
- [ ] Restrict creations, updates and deletions (only maintainers in the bypass list can create release tags).
- [ ] Block force pushes.

## Repository: security

Settings → Advanced Security (or "Code security")

- [ ] Private vulnerability reporting: **on**.
- [ ] Dependency graph: **on**.
- [ ] Dependabot alerts: **on**.
- [ ] Dependabot security updates: **on**.
- [ ] Secret scanning: **on**.
- [ ] Push protection: **on**.
- [ ] Code scanning: use the CodeQL workflow from the repo (not default setup), once it exists.

## Repository: packages

- [ ] After the first release, check that `ghcr.io/axyrasec/core-engine` is linked to this
      repository and that its visibility matches the repo.

## Personal accounts

- [ ] 2FA on every maintainer account.
- [ ] Commit signing key added on GitHub as a **Signing Key** (SSH or GPG).
- [ ] Git email matches a verified email on the GitHub account.
