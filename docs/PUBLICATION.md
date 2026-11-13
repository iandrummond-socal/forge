# Public release verification

Forge is distributed under MIT. Public source starts at commit
`340a307185e679d164638c5fde125821ea3c317b`, tagged **v0.3.1**.

Verified November 5, 2026:

- [Source CI](https://github.com/iandrummond-socal/forge/actions/runs/42114117565)
  passed all six jobs: Linux AMD64, Windows AMD64 and Intel macOS with Go 1.23.0
  and Go 1.27.3. Each native source gate includes 43 top-level race tests, vet,
  module verification, legacy compatibility, output/link/XML and rebuild checks.
- Local native macOS ARM64 validation passed with both Go versions. Six binary
  targets compiled. Packaged source passed offline tests with a fresh module cache.
- [Native packaged-binary checks](https://github.com/iandrummond-socal/forge/actions/runs/42115046953)
  passed on Linux AMD64, Windows AMD64 and Intel macOS. The packaged macOS ARM64
  executable also passed locally. Windows ARM64 remains compile-only.
- All 12 release artifacts and 181 unpacked members were checked for secret-scanner
  findings and known private operational identifiers. Gitleaks 8.30.1 also checked
  public Git history, the source tree and the three qualification Actions logs;
  these scans reported zero findings.
- Commits and annotated tags use the maintainer's GitHub noreply identity. The
  repository contains generic samples and no production account configuration,
  operational handoffs, personal-site content, credential files or Actions secrets.
- GitHub secret scanning and push protection are enabled. Local validation logs
  and machine paths are excluded from release packaging; only curated summaries
  are published. New releases must repeat the audit for their exact source/assets.

[Stable release](https://github.com/iandrummond-socal/forge/releases/tag/v0.3.1)
includes SHA256SUMS, exact source metadata, six binaries, source/sample archives
and license notices. Release tags/assets are immutable once published.

## v0.3.2

Source commit `88bc3fdeb3bc0fe1e5a355fe4a41ed1807cbe696`, tagged **v0.3.2**.
The change is the default theme's RSS nav link; the generator core is unchanged.

Verified November 5, 2026:

- [Source CI](https://github.com/iandrummond-socal/forge/actions/runs/42135984803)
  passed all six jobs on the release commit: Linux AMD64, Windows AMD64 and Intel
  macOS with Go 1.23.0 and Go 1.27.3.
- Local native macOS ARM64 validation passed on a clean tree with both Go versions
  (45 top-level race tests, vet, module verification, legacy compatibility,
  output/link/XML and rebuild checks). Six binary targets compiled.
- [Native packaged-binary checks](https://github.com/iandrummond-socal/forge/actions/runs/42145464677)
  passed on Linux AMD64, Windows AMD64 and Intel macOS. Windows ARM64 remains
  compile-only.
- The 12 published assets were downloaded again and match `SHA256SUMS`.
- The packaged source and sample archives were scanned with Gitleaks (zero
  findings) and the archives and binaries were searched for machine paths and
  personal identifiers (none). Git history and Actions logs were not rescanned
  for this release.

[Stable release](https://github.com/iandrummond-socal/forge/releases/tag/v0.3.2)
has the same asset set as v0.3.1.

## v0.4.0

Source commit `666316a9e2c37e6c94ef7f2583aced43b2873df5`, tagged **v0.4.0**.
The change is opt-in link-blog support: `link` front matter, timestamped dates,
rendered bodies in lists, and the `[home]` and `[feed]` tables. A site that uses
none of the new keys builds the same output as v0.3.2.

Verified November 12, 2026:

- [Source CI](https://github.com/iandrummond-socal/forge/actions/runs/43372560824)
  passed all six jobs on the release commit: Linux AMD64, Windows AMD64 and Intel
  macOS with Go 1.23.0 and Go 1.27.3.
- Local native macOS ARM64 validation passed on a clean tree with both Go versions
  (64 top-level race tests, vet, module verification, legacy compatibility,
  output/link/XML and rebuild checks, and the link-blog smoke). Six binary targets
  compiled.
- The v0.2 compatibility fixture builds byte-identical output to the v0.3.2
  release binary; the stored v0.3.2 output is compared on every test run.
- [Native packaged-binary checks](https://github.com/iandrummond-socal/forge/actions/runs/43373307184)
  passed on Linux AMD64, Windows AMD64 and Intel macOS. The packaged macOS ARM64
  executable also passed locally. Windows ARM64 remains compile-only.
- The 12 published assets were downloaded again; they match `SHA256SUMS` and the
  locally packaged files byte for byte.
- All 12 release artifacts and 191 unpacked members were scanned with Gitleaks
  (zero findings) and searched for machine paths and personal identifiers (none;
  the only path-like match is an example in the vendored TOML library's own help
  text, also present in earlier releases). Public Git history was scanned with
  Gitleaks (zero findings). Actions logs were not rescanned for this release.
- The v0.4.0 tag is a lightweight tag created with the GitHub release, pointing
  directly at the source commit; v0.3.1 and v0.3.2 are annotated tags. Commits use
  the maintainer's GitHub noreply identity.

[Stable release](https://github.com/iandrummond-socal/forge/releases/tag/v0.4.0)
has the same asset set as v0.3.2.
