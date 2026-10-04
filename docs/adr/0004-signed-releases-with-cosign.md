# 4. Sign releases with cosign (keyless)

- Status: Accepted
- Date: 2026-10-04

## Context

Users run our engine inside their trust boundary, between AI agents and their tools.
They need to know that the image they pull was built by us, from this repository,
and was not changed afterwards. Managing long-lived signing keys is risky and easy to get wrong.

## Decision

Every release is signed and comes with supply-chain metadata.

- Release images are pushed to `ghcr.io/axyrasec/core-engine`.
- Images are signed with [cosign](https://github.com/sigstore/cosign) in **keyless** mode:
  GitHub Actions OIDC identity, short-lived certificate from Fulcio, entry in the Rekor transparency log.
  No private signing key is stored anywhere.
- Each release has an SBOM (generated with syft) attached and attested.
- Each release has a build provenance attestation.
- Releases are built only by the release workflow, from a SemVer tag on `main`.

Users can verify an image with:

```sh
cosign verify ghcr.io/axyrasec/core-engine:<tag> \
  --certificate-identity-regexp '^https://github.com/AxyraSec/core-engine/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Consequences

- No signing key to store, rotate or leak.
- Anyone can check who built an image, from which workflow and which tag.
- Signatures are public in the Rekor log, including the repository name and workflow path.
- Releases depend on Sigstore public services being available at release time.
- The release workflow needs `id-token: write` and `packages: write`. These permissions
  are granted only to that job.
