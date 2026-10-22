#!/usr/bin/env python3
"""Fill an e2esshd SFTP root with the deploy tree the landing page's hero shot
shows: a release directory with enough entries to fill the files panel, and a
log long enough to fill the terminal under it.

    /tmp/e2esshd 2222 &            # prints "sftp root <path>"
    python3 tools/e2e/hero_fixture.py <path>

Everything is written under the path given. Timestamps are pinned to the day the
shot's logs describe, so the file list and the log agree with each other.
"""
import datetime
import os
import shutil
import sys

# The deployment the shot shows: releases on five different days, an upload
# from this afternoon, config from the day before. Every entry has its own time,
# the way a directory that has been lived in does.
DAY = datetime.datetime(2026, 10, 23, 18, 33, 0)


def day(month_day, hour=0, minute=0):
    month, day_of_month = month_day
    return datetime.datetime(2026, month, day_of_month, hour, minute)


def stamp(path, when):
    seconds = when.timestamp()
    os.utime(path, (seconds, seconds))


def write(root, name, content, when, size=None):
    path = os.path.join(root, name)
    directory = os.path.dirname(path)
    if directory:
        os.makedirs(directory, exist_ok=True)
    with open(path, "w") as f:
        f.write(content)
    if size and size > len(content):
        with open(path, "ab") as f:
            f.write(b"\0" * (size - len(content)))
    stamp(path, when)


def app_log():
    """Eighty lines of the service's log: a deploy, requests of several kinds,
    the worker, the cache and two warnings. Nothing 5xx, which is what the
    assistant's answer says the reader will not find."""
    lines = []
    start = datetime.datetime(2026, 10, 23, 18, 3, 14)
    steps = [5, 9, 6, 4, 11, 7, 5, 8, 6, 12, 5, 7]
    recipes = [
        "INFO  request served method=GET path=/api/v1/hosts status=200 ms={ms}",
        "INFO  request served method=GET path=/api/v1/hosts/41 status=101 ms=0",
        "INFO  request served method=POST path=/api/v1/sessions status=201 ms={ms}",
        "INFO  cache stats hit=0.93 keys=1204",
        "INFO  request served method=GET path=/healthz status=200 ms=1",
        "INFO  worker finished job=sessions_cleanup queue=0 took={ms}ms",
        "INFO  request served method=GET path=/api/v1/forwards status=200 ms={ms}",
        "INFO  worker finished job=metrics_flush queue=3 took={ms}ms",
        "INFO  request served method=POST path=/api/v1/forwards status=201 ms={ms}",
        "INFO  cache stats hit=0.91 keys=1198",
        "INFO  request served method=GET path=/assets/app.js status=200 ms=2",
    ]
    when = start
    for i in range(80):
        if i:
            when += datetime.timedelta(seconds=steps[i % len(steps)])
        clock = when.strftime("%H:%M:%S")
        if i == 0:
            body = "INFO  deploy finished release=2026-10-23 took=42s"
        elif i == 46:
            body = "WARN  slow query table=sessions ms=412"
        elif i == 52:
            body = "WARN  retry attempt=2 upstream=auth waited=1500ms"
        else:
            body = recipes[i % len(recipes)].format(ms=2 + (i * 13) % 17)
        lines.append(f"{clock} {body}")
    return "\n".join(lines) + "\n"


# The tree's own log viewer: the timestamp recedes, the level carries the colour,
# and status and latency are coloured by what they mean. The shot runs it, so the
# colours in the shell come from a file in the deploy tree, not from the sender.
LOGS_SH = r"""#!/bin/sh
# Colour the tail of a log by level, the way a log viewer does: the
# timestamp recedes, the level carries the colour, and the request line
# stands out from the values after it.
#   ./scripts/logs.sh [lines] [file]
set -eu
lines=${1:-32}
file=${2:-logs/app.log}
tail -n "$lines" "$file" | awk '
{
	if ($2 == "WARN") level = "33;1"
	else if ($2 == "ERROR") level = "31;1"
	else level = "32"
	rest = substr($0, index($0, $3))
	sub(/path=[^ ]+/, "\033[36m&\033[0m", rest)
	sub(/status=[0-9]+/, "\033[2m&\033[0m", rest)
	printf "\033[2m%s\033[0m \033[%sm%-5s\033[0m %s\n", $1, level, $2, rest
}'
"""


def main():
    if len(sys.argv) != 2:
        sys.exit("usage: hero_fixture.py <sftp root>")
    root = sys.argv[1]

    # The fixture ships alpha.txt and docs/ for the e2e tests; this script owns
    # the root, and the shot's tree replaces them, so a second run is the same
    # picture as the first.
    for name in os.listdir(root):
        path = os.path.join(root, name)
        if os.path.isdir(path) and not os.path.islink(path):
            shutil.rmtree(path)
        else:
            os.remove(path)

    # Directories first: their names are what the panel groups at the top.
    for name in ("backups", "config", "docker", "logs", "public", "releases", "scripts", "storage"):
        os.makedirs(os.path.join(root, name), exist_ok=True)

    write(root, "README.md", "# staging-web\n\nDeploy notes for the public API. Read docs/ before rolling back.\n", day((9, 28), 10, 12))
    write(root, ".env.example", "APP_ENV=production\nAPP_PORT=8080\nDATABASE_URL=postgres://app@db:5432/app\nREDIS_URL=redis://cache:6379/0\n", day((10, 18), 9, 33))
    write(root, ".env.production", "APP_ENV=production\nAPP_PORT=8080\nDATABASE_URL=postgres://app@db:5432/app\n", day((10, 18), 9, 33))
    write(root, "CHANGELOG.md", "## 2026-10-23\n- rate limit the login route\n- fix slow session lookup\n\n## 2026-10-22\n- add /api/v1/hosts\n", day((10, 23), 17, 44))
    write(root, "LICENSE", "MIT License\n\nCopyright (c) 2026 staging-web\n", day((9, 28), 10, 12))
    write(root, "VERSION", "1.4.2\n", day((10, 23), 18, 2))
    write(root, "Makefile", "deploy:\n\tsh deploy.sh\n\nrollback:\n\tsh rollback.sh\n", day((10, 21), 11, 4))
    write(root, "compose.yaml", "services:\n  app:\n    image: registry.local/staging-web:2026-10-23\n  worker:\n    image: registry.local/staging-web:2026-10-23\n", day((10, 18), 9, 33))
    write(root, "crontab", "0 3 * * * /srv/app/backup.sh\n*/5 * * * * /srv/app/healthcheck.sh\n", day((10, 18), 9, 33))
    write(root, "deploy.sh", "#!/bin/sh\nset -eu\nrelease=$(date +%F)\ntar -czf releases/$release.tar.gz app\nln -sfn releases/$release current\nsystemctl reload staging-web\n", day((10, 21), 11, 4))
    write(root, "rollback.sh", "#!/bin/sh\nset -eu\nln -sfn releases/$1 current\nsystemctl reload staging-web\n", day((10, 21), 11, 4))
    write(root, "package.json", '{\n  "name": "staging-web",\n  "private": true,\n  "scripts": {"start": "node server.js"}\n}\n', day((10, 18), 9, 33))
    write(root, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n", day((10, 18), 9, 33))
    write(root, "tsconfig.json", '{\n  "compilerOptions": {"target": "ES2022", "strict": true}\n}\n', day((10, 18), 9, 33))
    write(root, "ecosystem.config.js", "module.exports = { apps: [{ name: 'api', script: 'server.js', instances: 4 }] }\n", day((10, 18), 9, 33))
    write(root, "robots.txt", "User-agent: *\nDisallow: /admin\n", day((10, 18), 9, 33))
    write(root, ".dockerignore", "node_modules\nstorage/uploads\n", day((10, 18), 9, 33))
    write(root, ".editorconfig", "root = true\n\n[*]\nindent_style = space\nindent_size = 2\n", day((9, 28), 10, 12))
    write(root, ".gitignore", "node_modules\n.env\nstorage/cache\n", day((10, 18), 9, 33))
    write(root, "eslint.config.js", "export default [{ rules: { \"no-console\": \"warn\" } }];\n", day((10, 21), 11, 4))
    write(root, "prettier.config.js", "export default { printWidth: 100, singleQuote: true };\n", day((10, 21), 11, 4))
    write(root, "server.js", "import { createServer } from './app/http.js';\ncreateServer().listen(8080);\n", day((10, 23), 18, 2))
    write(root, "worker.js", "import { run } from './app/jobs.js';\nrun();\n", day((10, 23), 18, 2))

    for name in ("nginx.conf", "app.yaml", "logging.yaml", "redis.conf"):
        write(root, f"config/{name}", f"# {name}\n", day((10, 22), 16, 47))
    for name in ("Dockerfile", "entrypoint.sh", "healthcheck.sh"):
        write(root, f"docker/{name}", f"# {name}\n", day((10, 18), 9, 33))
    for name in ("backup.sh", "migrate.sh", "restart.sh", "rollback.sh", "seed.sh"):
        write(root, f"scripts/{name}", f"# {name}\n", day((10, 21), 11, 4))
    write(root, "scripts/logs.sh", LOGS_SH, day((10, 21), 11, 4))
    os.chmod(os.path.join(root, "scripts/logs.sh"), 0o755)
    current = os.path.join(root, "current")
    os.symlink("releases/2026-10-23", current)
    released = day((10, 23), 18, 2).timestamp()
    os.utime(current, (released, released), follow_symlinks=False)
    for name in ("favicon.ico", "index.html", "manifest.json"):
        write(root, f"public/{name}", "x", day((10, 18), 9, 36))
    for name in ("uploads/avatar-1042.png", "uploads/avatar-1187.png", "cache/thumb-8891.png"):
        write(root, f"storage/{name}", "x", day((10, 23), 17, 58), size=2_400)
    for name, when in (("db-2026-10-22.sql.gz", day((10, 22), 3, 10)), ("db-2026-10-23.sql.gz", day((10, 23), 3, 10)), ("redis-2026-10-23.rdb", day((10, 23), 3, 11))):
        write(root, f"backups/{name}", "x", when, size=171_000)
    for name, size, when in (
        ("2026-10-19", 168_000, day((10, 19), 18, 4)),
        ("2026-10-20", 171_000, day((10, 20), 18, 9)),
        ("2026-10-22", 179_000, day((10, 22), 18, 2)),
        ("2026-10-23", 183_000, day((10, 23), 18, 2)),
    ):
        write(root, f"releases/{name}.tar.gz", "x", when, size=size)
        write(root, f"releases/{name}/REVISION", f"{name}\n", when)

    write(root, "logs/app.log", app_log(), day((10, 23), 18, 20))
    write(root, "logs/access.log", app_log()[:1400], day((10, 23), 18, 20), size=48_000)
    write(root, "logs/error.log", "2026-10-23 18:08:22 WARN  slow query table=sessions 412ms\n", day((10, 23), 18, 20), size=900)
    write(root, "logs/deploy.log", "2026-10-23 18:02:11 deploy 2026-10-23 finished in 42s\n", day((10, 23), 18, 20), size=1_300)

    # The directories themselves, last: the panel draws their own dates beside
    # their names.
    for name, when in (
        ("backups", day((10, 23), 3, 11)),
        ("config", day((10, 22), 16, 47)),
        ("docker", day((10, 18), 9, 33)),
        ("logs", day((10, 23), 18, 20)),
        ("public", day((10, 18), 9, 36)),
        ("releases", day((10, 23), 18, 2)),
        ("scripts", day((10, 21), 11, 4)),
        ("storage", day((10, 23), 17, 58)),
    ):
        stamp(os.path.join(root, name), when)


if __name__ == "__main__":
    main()
