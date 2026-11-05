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
