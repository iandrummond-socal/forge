#!/usr/bin/env python3
"""Build deterministic local artifacts from a clean, validated Git commit. No publish/deploy."""
import argparse
from datetime import datetime, timezone
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
TARGETS = [("darwin", "arm64"), ("darwin", "amd64"), ("linux", "amd64"),
           ("linux", "arm64"), ("windows", "amd64"), ("windows", "arm64")]


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def archive(path, prefix, entries, epoch):
    """Normalized timestamps, permissions, owners, ordering and gzip header."""
    if path.suffix == ".zip":
        date = datetime.fromtimestamp(max(epoch, 315532800), timezone.utc).timetuple()[:6]
        with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as out:
            for name, data, mode in sorted(entries):
                info = zipfile.ZipInfo(prefix + "/" + name, date)
                info.create_system = 3
                info.external_attr = (stat.S_IFREG | mode) << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                out.writestr(info, data, compresslevel=9)
    else:
        with path.open("wb") as raw, gzip.GzipFile(filename="", fileobj=raw, mode="wb", mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.PAX_FORMAT) as out:
                for name, data, mode in sorted(entries):
                    info = tarfile.TarInfo(prefix + "/" + name)
                    info.size, info.mode, info.mtime = len(data), mode, epoch
                    info.uid = info.gid = 0
                    info.uname = info.gname = ""
                    out.addfile(info, io.BytesIO(data))


def entries_from(root):
    return [(p.relative_to(root).as_posix(), p.read_bytes(), 0o755 if p.stat().st_mode & 0o111 else 0o644)
            for p in root.rglob("*") if p.is_file()]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", default="v0.3.1")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--validation", type=Path, nargs="+", required=True)
    args = parser.parse_args()
    if not re.fullmatch(r"v\d+\.\d+\.\d+(?:-[a-zA-Z0-9.]+)?", args.version):
        parser.error("version must be a simple v-prefixed semantic version")
    output = (args.output or ROOT / "release" / args.version).resolve()
    if output.exists():
        parser.error("output already exists; choose a new directory (nothing is overwritten)")
    env = dict(os.environ, GOTOOLCHAIN="local", GOPROXY="off", GOSUMDB="off", GOWORK="off", GOFLAGS="")

    def run(command, cwd=ROOT, override=None):
        result = subprocess.run([str(x) for x in command], cwd=cwd, env=dict(env, **(override or {})),
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        if result.returncode:
            raise RuntimeError(f"{command} failed:\n{result.stdout}")
        return result.stdout.strip()

    if run(["git", "status", "--porcelain"]):
        raise RuntimeError("commit or preserve outstanding work before packaging; working tree must be clean")
    commit = run(["git", "rev-parse", "HEAD"])
    epoch = int(run(["git", "show", "-s", "--format=%ct", "HEAD"]))
    toolchain = run(["go", "version"])
    host = run(["go", "env", "GOHOSTOS", "GOHOSTARCH"]).splitlines()
    reports = [json.loads(path.read_text()) for path in args.validation]
    for report in reports:
        if report.get("status") != "passed" or report.get("source_commit") != commit or report.get("working_tree_dirty") is not False:
            raise RuntimeError("each report must pass on this exact clean commit")
    versions = {report["go_version"] for report in reports
                if [report.get("platform", {}).get("GOOS"), report.get("platform", {}).get("GOARCH")] == host}
    if not any(v.startswith("go version go1.23.0 ") for v in versions) or len(versions) < 2 or toolchain not in versions:
        raise RuntimeError("require native-host Go 1.23.0 and a second toolchain report, including the packaging toolchain")
    metadata = {"version": args.version, "source_commit": commit, "source_epoch": epoch,
                "toolchain": toolchain, "status": "local release build; publication separate; no deployment"}
    metadata_bytes = (json.dumps(metadata, indent=2, sort_keys=True) + "\n").encode()
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".forge-release-", dir=output.parent) as working:
        work = Path(working)
        source, products = work / "source", work / "artifacts"
        source.mkdir(); products.mkdir()
        # Export the commit, never untracked files or original downloaded binaries.
        exported = subprocess.check_output(["git", "archive", "--format=tar", commit], cwd=ROOT)
        with tarfile.open(fileobj=io.BytesIO(exported)) as tar:
            for member in tar.getmembers():
                relative = Path(member.name)
                if relative.is_absolute() or ".." in relative.parts or not (member.isfile() or member.isdir()):
                    raise RuntimeError(f"unsupported source archive member: {member.name}")
                destination = source / relative
                if member.isdir():
                    destination.mkdir(parents=True, exist_ok=True)
                else:
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    destination.write_bytes(tar.extractfile(member).read())
                    destination.chmod(member.mode & 0o777)
        run(["go", "mod", "vendor"], source)
        (source / "RELEASE.json").write_bytes(metadata_bytes)
        prefix = "forge-" + args.version
        archive(products / (prefix + "-source.zip"), prefix + "-source", entries_from(source), epoch)
        sample_entries = entries_from(source / "examples/sample")
        sample_entries += [("LICENSE", (source / "LICENSE").read_bytes(), 0o644),
                           ("README.md", (source / "docs/SAMPLE_README.md").read_bytes(), 0o644),
                           ("RELEASE.json", metadata_bytes, 0o644)]
        archive(products / (prefix + "-sample.zip"), prefix + "-sample", sample_entries, epoch)
        notices = [("licenses/BurntSushi-toml.txt", (source / "vendor/github.com/BurntSushi/toml/COPYING").read_bytes(), 0o644),
                   ("licenses/goldmark.txt", (source / "vendor/github.com/yuin/goldmark/LICENSE").read_bytes(), 0o644)]
        manifest = dict(metadata, platforms=[], validation=[])
        flags = f"-buildid= -X github.com/iandrummond-socal/forge/internal/cli.Version={args.version} -X github.com/iandrummond-socal/forge/internal/cli.Commit={commit}"
        for system, architecture in TARGETS:
            target = system + "-" + architecture
            target_env = {"GOOS": system, "GOARCH": architecture, "CGO_ENABLED": "0"}
            binary = work / ("forge.exe" if system == "windows" else "forge")
            run(["go", "build", "-mod=vendor", "-trimpath", "-buildvcs=false", "-ldflags", flags,
                 "-o", binary, "./cmd/forge"], source, target_env)
            testdir = work / ("tests-" + target); testdir.mkdir()
            run(["go", "test", "-mod=vendor", "-c", "-o", testdir, "./..."], source, target_env)
            native = [system, architecture] == host
            if native:
                actual = run([binary, "version"])
                assert actual == f"forge {args.version} (commit {commit})", actual
            extension = ".zip" if system == "windows" else ".tar.gz"
            archive(products / (prefix + "-" + target + extension), prefix + "-" + target,
                    [(binary.name, binary.read_bytes(), 0o755), ("LICENSE", (source / "LICENSE").read_bytes(), 0o644),
                     ("RELEASE.json", metadata_bytes, 0o644)] + notices, epoch)
            manifest["platforms"].append({"target": target, "binary_sha256": digest(binary),
                                          "cli_and_tests_compiled": True, "version_executed": native})
            print("PACKAGED", target, "(native executed)" if native else "(cross-compiled only)", flush=True)
        # Validate the actual exported, vendored source archive in a fresh module cache.
        unpack = work / "unpack"; unpack.mkdir()
        with zipfile.ZipFile(products / (prefix + "-source.zip")) as zipped:
            zipped.extractall(unpack)
        extracted = unpack / (prefix + "-source")
        run(["go", "test", "-mod=vendor", "-count=1", "./..."], extracted,
            {"GOOS": host[0], "GOARCH": host[1], "GOMODCACHE": str(work / "empty-module-cache")})
        manifest["source_archive_offline_tests"] = "passed with fresh module cache and GOPROXY=off"
        # Execute the actual packaged native CLI, not only the staging binary.
        native_name = prefix + "-" + "-".join(host)
        native_path = products / (native_name + (".zip" if host[0] == "windows" else ".tar.gz"))
        if native_path.suffix == ".zip":
            with zipfile.ZipFile(native_path) as zipped:
                zipped.extractall(unpack)
        else:
            with tarfile.open(native_path) as tar:
                tar.extractall(unpack)  # This archive was just constructed from fixed file names above.
        executable = unpack / native_name / ("forge.exe" if host[0] == "windows" else "forge")
        executable.chmod(0o755)
        run([executable, "version"])
        with zipfile.ZipFile(products / (prefix + "-sample.zip")) as zipped:
            zipped.extractall(unpack)
        sample = unpack / (prefix + "-sample")
        for command in ("check", "build"):
            run([executable, command, "-root", sample, "-as-of", "2026-11-04T00:00:00Z"])
        from validate import inspect_site
        manifest["packaged_native_sample_smoke"] = inspect_site(sample / "dist")
        # Publish only curated summaries; raw reports/logs may contain local paths.
        for i, report in enumerate(reports, 1):
            name = f"validation-{i}.json"
            summary = {"status": report["status"], "source_commit": commit,
                       "working_tree_dirty": False, "go_version": report["go_version"],
                       "top_level_tests_passed": report["top_level_tests_passed"]}
            (products / name).write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
            manifest["validation"].append({"file": name, "go_version": report["go_version"],
                                            "top_level_tests_passed": report["top_level_tests_passed"]})
        manifest["artifacts"] = {p.name: digest(p) for p in sorted(products.iterdir())}
        (products / "MANIFEST.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
        (products / "SHA256SUMS").write_text("".join(f"{digest(p)}  {p.name}\n" for p in sorted(products.iterdir())))
        # Final byte-level verification before exposing an output directory.
        for line in (products / "SHA256SUMS").read_text().splitlines():
            checksum, name = line.split("  ", 1)
            assert digest(products / name) == checksum, name
        products.rename(output)
    print("Verified local artifacts:", output, flush=True)


if __name__ == "__main__":
    main()
