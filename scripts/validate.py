#!/usr/bin/env python3
"""Offline release checks after Go modules have been downloaded; never deploys."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
from html.parser import HTMLParser
from urllib.parse import unquote, urljoin, urlsplit
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
AS_OF = "2026-11-04T00:00:00Z"


def tree_hashes(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob("*")) if p.is_file()}


class Links(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links, self.ids = [], set()

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if "id" in attrs:
            self.ids.add(attrs["id"])
        for key in ("href", "src"):
            if attrs.get(key):
                self.links.append(attrs[key])


def inspect_site(output):
    """Verify HTML links/assets/fragments and sitemap targets, without networking."""
    documents = {}
    for path in output.rglob("*.html"):
        parser = Links()
        html = path.read_text()
        assert "EventSource" not in html, f"preview script in {path}"
        parser.feed(html)
        documents[path.resolve()] = parser
    checked = 0
    for path, document in documents.items():
        relative = path.relative_to(output.resolve()).as_posix()
        page_url = "/" + relative.removesuffix("index.html")
        for link in document.links:
            parsed = urlsplit(urljoin("https://forge.invalid" + page_url, link))
            if parsed.netloc != "forge.invalid" or parsed.scheme not in ("http", "https"):
                continue
            target = (output / unquote(parsed.path).lstrip("/")).resolve()
            assert target.is_relative_to(output.resolve()), f"link escapes output: {link}"
            if target.is_dir():
                target = target / "index.html"
            assert target.is_file(), f"broken local link {link} in {path}"
            if parsed.fragment and target in documents:
                assert unquote(parsed.fragment) in documents[target].ids, f"missing fragment {link} in {path}"
            checked += 1
    for name in ("rss.xml", "sitemap.xml"):
        ET.parse(output / name)
    for loc in ET.parse(output / "sitemap.xml").iter("{http://www.sitemaps.org/schemas/sitemap/0.9}loc"):
        target = output / unquote(urlsplit(loc.text).path).lstrip("/") / "index.html"
        assert target.is_file(), f"missing sitemap target: {loc.text}"
    return {"files": len(tree_hashes(output)), "html_documents": len(documents), "local_links_checked": checked}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", type=Path, default=ROOT / "release/validation/report.json")
    args = parser.parse_args()
    report_path = args.report.resolve()
    report_path.parent.mkdir(parents=True, exist_ok=True)
    logs = report_path.parent / (report_path.stem + "-logs")
    logs.mkdir(exist_ok=True)
    report = {"status": "running", "commands": [], "smoke": {}, "as_of": AS_OF,
              "scope": "native race tests/vet; bundled-site links and determinism; no network link or deployment checks"}
    env = dict(os.environ, GOTOOLCHAIN="local", GOPROXY="off", GOSUMDB="off")

    def run(command, cwd=ROOT):
        result = subprocess.run([str(x) for x in command], cwd=cwd, env=env,
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        log = logs / f"{len(report['commands']) + 1:02d}.log"
        log.write_text(result.stdout)
        report["commands"].append({"argv": [str(x) for x in command], "exit_code": result.returncode, "log": log.name})
        if result.returncode:
            raise RuntimeError(f"{command} failed; see {log}\n{result.stdout[-4000:]}")
        return result.stdout

    try:
        if (ROOT / ".git").exists():
            report["source_commit"] = run(["git", "rev-parse", "HEAD"]).strip()
            report["working_tree_dirty"] = bool(run(["git", "status", "--porcelain"]).strip())
        else:
            report["source_commit"] = json.loads((ROOT / "RELEASE.json").read_text())["source_commit"]
            report["working_tree_dirty"] = False
        report["go_version"] = run(["go", "version"]).strip()
        report["platform"] = json.loads(run(["go", "env", "-json", "GOOS", "GOARCH", "GOHOSTOS", "GOHOSTARCH"]))
        platform = report["platform"]
        report["execution_mode"] = "native" if (platform["GOOS"], platform["GOARCH"]) == (platform["GOHOSTOS"], platform["GOHOSTARCH"]) else "OS compatibility layer"
        print(report["go_version"], platform["GOOS"] + "/" + platform["GOARCH"], report["execution_mode"], flush=True)
        tests = run(["go", "test", "-race", "-count=1", "-json", "./..."])
        events = [json.loads(line) for line in tests.splitlines() if line.startswith("{")]
        report["top_level_tests_passed"] = sum(e.get("Action") == "pass" and bool(e.get("Test")) and "/" not in e["Test"] for e in events)
        run(["go", "vet", "./..."])
        run(["go", "mod", "verify"])
        with tempfile.TemporaryDirectory(prefix="forge-release-validation-") as directory:
            temp = Path(directory)
            binary = temp / ("forge.exe" if os.name == "nt" else "forge")
            run(["go", "build", "-trimpath", "-buildvcs=false", "-o", binary, "./cmd/forge"])
            report["cli_version"] = run([binary, "version"]).strip()
            fresh = temp / "fresh"
            run([binary, "init", fresh])
            legacy, sample = temp / "legacy", temp / "sample"
            for source, target in ((ROOT / "testdata/v0.2-site", legacy), (ROOT / "examples/sample", sample)):
                shutil.copytree(source, target, ignore=shutil.ignore_patterns("dist", ".DS_Store", ".wrangler", "node_modules"))
            for name, site in (("legacy_v0.2", legacy), ("fresh", fresh), ("sample", sample)):
                for command in ("check", "build"):
                    run([binary, command, "-root", site, "-as-of", AS_OF])
                before = tree_hashes(site / "dist")
                run([binary, "build", "-root", site, "-as-of", AS_OF])
                assert before == tree_hashes(site / "dist"), f"nondeterministic output: {name}"
                result = inspect_site(site / "dist")
                assert result["files"] == (6 if name == "legacy_v0.2" else 17), result
                report["smoke"][name] = dict(result, deterministic=True)
            # Force multiple pages so the release gate checks real pagination links.
            config = (fresh / "site.toml").read_text().replace("page_size = 10", "page_size = 1")
            (fresh / "site.toml").write_text(config)
            run([binary, "build", "-root", fresh, "-as-of", AS_OF])
            assert (fresh / "dist/page/2/index.html").is_file()
            report["smoke"]["paginated"] = inspect_site(fresh / "dist")
        report["status"] = "passed"
        print(f"PASS: {report['top_level_tests_passed']} tests, vet, module verification, compatibility, local links and deterministic builds", flush=True)
    except Exception as error:
        report["status"] = "failed"
        report["error"] = str(error)
        raise
    finally:
        report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
