# Releasing

1. Add a `## [X.Y.Z] - YYYY-MM-DD` section to `CHANGELOG.md` (it becomes the release notes)
   and update the links at the bottom. Commit and push to `main`.
2. Wait for CI and Security to be green on that commit.
3. Tag and push:

   ```sh
   git tag -s vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

4. The Release workflow builds the binaries (linux, macOS, windows × amd64, arm64), pushes
   and signs the image, signs `checksums.txt`, and creates the GitHub Release with
   `core-engine.rb` attached.
5. Update Homebrew: copy `core-engine.rb` from the release to `Formula/core-engine.rb` in
   the `AxyraSec/homebrew-tap` repository, commit and push.

Check locally before tagging: `make dist VERSION=vX.Y.Z` builds the same binaries into `dist/`.

## Homebrew tap (one-time setup)

Create a public repository `AxyraSec/homebrew-tap` with a `Formula/` folder.
Users then install with `brew install axyrasec/tap/core-engine`.

Homebrew and `install.sh` download from public release URLs: they only work for people
outside the organization once the releases are public.
