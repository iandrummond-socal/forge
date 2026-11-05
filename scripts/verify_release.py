#!/usr/bin/env python3
"""Verify downloaded release bytes and execute the matching native CLI/sample."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import socket
import subprocess
import tarfile
import tempfile
import time
import urllib.request
import zipfile
from validate import inspect_site, tree_hashes


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--assets', type=Path, required=True)
    parser.add_argument('--version', required=True)
    parser.add_argument('--source', required=True)
    parser.add_argument('--report', type=Path, required=True)
    args = parser.parse_args()
    assets = args.assets.resolve()
    manifest = json.loads((assets / 'MANIFEST.json').read_text())
    if manifest['version'] != args.version or manifest['source_commit'] != args.source:
        raise RuntimeError('release version/source mismatch')
    for line in (assets / 'SHA256SUMS').read_text().splitlines():
        expected, name = line.split('  ', 1)
        if Path(name).name != name or hashlib.sha256((assets/name).read_bytes()).hexdigest() != expected:
            raise RuntimeError('checksum mismatch or unsafe name: ' + name)
    system = {'Darwin': 'darwin', 'Linux': 'linux', 'Windows': 'windows'}[platform.system()]
    arch = {'arm64': 'arm64', 'aarch64': 'arm64', 'amd64': 'amd64', 'x86_64': 'amd64'}[platform.machine().lower()]
    prefix = 'forge-' + args.version
    native = prefix + '-' + system + '-' + arch
    binary_archive = assets / (native + ('.zip' if system == 'windows' else '.tar.gz'))
    report = {'status':'running', 'source_commit':args.source, 'version':args.version,
              'platform':system+'/'+arch, 'all_asset_checksums':'passed'}
    try:
        with tempfile.TemporaryDirectory(prefix='forge-native-release-') as d:
            root = Path(d)
            for archive in [binary_archive, assets/(prefix+'-sample.zip')]:
                if archive.suffix == '.zip':
                    with zipfile.ZipFile(archive) as z:
                        for info in z.infolist():
                            path = Path(info.filename)
                            if path.is_absolute() or '..' in path.parts:
                                raise RuntimeError('unsafe ZIP member')
                        z.extractall(root)
                else:
                    with tarfile.open(archive) as tar:
                        for member in tar.getmembers():
                            path = Path(member.name)
                            if path.is_absolute() or '..' in path.parts or not member.isfile():
                                raise RuntimeError('unsafe tar member')
                            target = root/path
                            target.parent.mkdir(parents=True,exist_ok=True)
                            target.write_bytes(tar.extractfile(member).read())
                            target.chmod(member.mode & 0o777)
            binary = root/native/('forge.exe' if system == 'windows' else 'forge')
            if os.name != 'nt': binary.chmod(0o755)
            def run(*command):
                return subprocess.check_output([str(binary), *map(str,command)], text=True, stderr=subprocess.STDOUT).strip()
            report['cli_version'] = run('version')
            if report['cli_version'] != f'forge {args.version} (commit {args.source})':
                raise RuntimeError('binary linker version/source mismatch')
            sample = root/(prefix+'-sample')
            run('check','-root',sample,'-as-of','2026-11-04T00:00:00Z')
            run('build','-root',sample,'-as-of','2026-11-04T00:00:00Z')
            before = tree_hashes(sample/'dist')
            report['sample'] = inspect_site(sample/'dist')
            run('build','-root',sample,'-as-of','2026-11-04T00:00:00Z')
            if before != tree_hashes(sample/'dist'): raise RuntimeError('nondeterministic packaged build')
            with socket.socket() as sock:
                sock.bind(('127.0.0.1',0));port = sock.getsockname()[1]
            process = subprocess.Popen([str(binary),'serve','-root',str(sample),'-watch=false','-addr',f'127.0.0.1:{port}'],stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
            try:
                deadline = time.monotonic()+15
                while True:
                    try:
                        with urllib.request.urlopen(f'http://127.0.0.1:{port}/',timeout=1) as response:
                            html = response.read().decode()
                        if 'EventSource' not in html: raise RuntimeError('preview reload injection absent')
                        break
                    except (OSError, TimeoutError):
                        if process.poll() is not None or time.monotonic() > deadline:
                            raise RuntimeError('packaged preview did not start')
                        time.sleep(.1)
                with urllib.request.urlopen(f'http://127.0.0.1:{port}/tags/go/',timeout=3) as response:
                    if response.status != 200: raise RuntimeError('taxonomy preview failed')
                report['http_preview'] = 'homepage and taxonomy passed on loopback'
            finally:
                process.terminate()
                try: process.communicate(timeout=5)
                except subprocess.TimeoutExpired: process.kill();process.communicate()
            if before != tree_hashes(sample/'dist'): raise RuntimeError('preview modified deployable output')
        report['status']='passed'
        print(json.dumps(report,sort_keys=True),flush=True)
    except Exception as error:
        report['status']='failed';report['error']=str(error);raise
    finally:
        args.report.parent.mkdir(parents=True,exist_ok=True)
        args.report.write_text(json.dumps(report,indent=2,sort_keys=True)+'\n')


if __name__ == '__main__': main()
