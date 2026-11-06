# Release procedure

Use a clean source commit, Python 3.11+, and cached Go modules. Run
`python3 scripts/validate.py --report release/validation/minimum.json` with Go
1.23.0, and again with a second Go toolchain into a separate report path.

Package with the second toolchain:

```sh
python3 scripts/package_release.py --version v0.3.2 \
  --validation release/validation/minimum.json release/validation/current.json
```

Reports must pass on the exact clean commit and native host. Packaging builds
six targets with trimpath, vendors dependencies, tests the source offline, checks
the packaged native CLI/sample, and emits checksums and dependency licenses.
Only curated validation summaries are distributed; raw local logs/reports stay
under ignored release/. Inspect and secret-scan all artifacts before publishing.
Use the read-only Verify release binaries workflow for actual native artifact
checks after an authorized release upload. Never change published tag/assets.

Public commits use the maintainer GitHub noreply address. Keep account IDs,
personal email, local filesystem paths, credentials and operational handoffs out
of tracked files and release assets. Neither validation nor packaging deploys.
