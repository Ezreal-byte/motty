# Tools

Developer-only helpers. Nothing here is compiled into the application or shipped
in a package.

## Entry points

| Path | What it does |
| --- | --- |
| `headless_drive.py` | Drives a `-tags vivahead` build through the headless line protocol: `click`, `type`, `key`, `waitfor`, `shot`, `texts`, `layout`. `--build` and `--watch` add the `dev` tag, so the interface they iterate has the component gallery. |
| `dev.sh` | Rebuilds and restarts a real window when Go files here or in `../viva` change. Needs `fswatch`. Builds with `-tags dev`, which is what adds the component gallery: the release build has no such page. |
| `release.sh` | Builds the release packages, checksums, signing and optional upload. See `docs/release.md`. |
| `competitor_issues.py` | Fetches closed issues (and pull requests, dropped) from the Netcatty and NyaTerm repositories into JSON for requirement mining. `--state`, `--label`, `--summary`. |

Run the first two from the repository root.

## End-to-end

`e2e/` holds the headless command scripts. They are not Go tests: each one names
its fixture, environment variables and commands in its header. Run them from the
repository root, for example:

```sh
go build -tags vivahead -o /tmp/motty-head ./cmd/motty
(cd tools/e2esshd && go build -o /tmp/e2esshd . && /tmp/e2esshd 2222 &)
python3 tools/headless_drive.py "$(mktemp -d)" tools/e2e/tray_e2e.txt
```

`e2e/mcp_e2e.py` is the Python harness for the MCP server and stdio bridge; it
also expects the loopback SSH fixture on port 2222 and a built `/tmp/motty-head`.

Fixtures:

- `e2esshd/` — loopback-only SSH/SFTP/SCP fixture with its own Go module.
- `fakes3/` — loopback S3 endpoint for the cloud-sync scripts.

## Live servers

`live/` runs the RFB and RDP tests against real servers rather than fixtures:

- `live/vnc_live.sh` — TigerVNC in Docker, or KasmVNC on the shared Linux test
  machine.
- `live/rdp_live.sh` — xrdp in Docker, or the Windows test machine.
- `live/vnc/` and `live/rdp/` — container contexts and helper scripts used by the
  two commands.

Docker builds these contexts; the scripts remove their containers and images when
they finish.
