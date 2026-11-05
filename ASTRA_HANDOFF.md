# Forge public development handoff

Module/repository: github.com/iandrummond-socal/forge. MIT license.
Public release series starts at v0.3.1; preserve v0.2 compatibility and v0.3
publishing semantics. Read README.md, docs/roadmap.md and docs/RELEASE.md.

Architecture: cmd/forge CLI; internal/site metadata, publishing and indexes;
internal/markup Goldmark; internal/build templates/assets/feeds; internal/preview
polling/SSE. examples/sample is generic; testdata/v0.2-site is the legacy fixture.

Run go mod download, go test -race ./..., go vet ./..., then
python3 scripts/validate.py. CI validates the minimum and current Go toolchains
on Linux, Windows and Intel macOS. Packaging requires exact-source clean reports.

No production infrastructure or personal site content belongs in this repository.
Do not commit credentials, local/private handoffs, account IDs or machine paths.
Use GitHub noreply commit identity. Keep public documentation generic.

Public v0.3.1 is published/latest. See docs/PUBLICATION.md for verified CI, native
binary checks and privacy audit scope. The release source is 340a307185e679d164638c5fde125821ea3c317b;
subsequent documentation commits do not retag or replace released bytes.
