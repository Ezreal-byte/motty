"""Write the signed-package feed for this fork's Windows acceptance build."""
import hashlib
import json
import pathlib
import sys

version, directory = sys.argv[1], pathlib.Path(sys.argv[2])
name = f"motty-{version}-windows-amd64-setup.exe"
package = directory / name
signatures = dict(line.split("\t", 1) for line in (directory / "SIGNATURES").read_text().splitlines())
manifest = {
    "version": version,
    "tag": f"v{version}",
    "source": f"https://github.com/Ezreal-byte/motty/releases/tag/v{version}",
    "notes": f"升级验收版本 {version}。验证自定义目录原地升级、签名校验、自动重启及配置保留。",
    "files": [{
        "name": "motty-windows-amd64-setup.exe",
        "size": package.stat().st_size,
        "sha256": hashlib.sha256(package.read_bytes()).hexdigest(),
        "signature": signatures[name],
    }],
}
(directory / "latest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
