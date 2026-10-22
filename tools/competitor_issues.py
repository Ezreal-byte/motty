#!/usr/bin/env python3
"""Fetch closed issues from the two competitor repositories for requirement mining.

    python3 tools/competitor_issues.py --out /tmp/mine
    python3 tools/competitor_issues.py --label enhancement --out /tmp/mine
    python3 tools/competitor_issues.py --repo nyakang/nyaterm --state closed --summary

Writes one JSON file per repository (`<owner>-<name>.json`): number, title, body,
state, state_reason, timestamps, comment and reaction counts, labels, author.
Pull requests are dropped, because `/issues` returns them mixed in and on an
active repository they are most of the payload.

GitHub caps a single `/issues` query at 1000 results. When a window fills up the
window is split at the newest `created_at` and each half is fetched again, so the
whole history comes back without a token. Set GITHUB_TOKEN to lift the 60
requests per hour unauthenticated limit; the run says how many requests are left.

`--summary` only prints counts (by state_reason and by label) for a directory of
already fetched files. Nothing here is compiled into the application.
"""
import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

REPOS = ("binaricat/Netcatty", "nyakang/nyaterm")
API = "https://api.github.com"
PER_PAGE = 100
# GitHub refuses page * per_page > 1000 for this endpoint.
WINDOW_LIMIT = 1000


def request(path, params):
    url = API + path + "?" + urllib.parse.urlencode(params)
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "motty-competitor-issues",
    }
    token = os.environ.get("GITHUB_TOKEN")
    if token:
        headers["Authorization"] = "Bearer " + token
    for attempt in range(4):
        try:
            with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=30) as reply:
                return json.load(reply), dict(reply.headers)
        except urllib.error.HTTPError as error:
            if error.code == 422:
                # Window too large: the caller bisects.
                return None, {}
            if error.code in (403, 429) and attempt < 3:
                wait = int(error.headers.get("X-RateLimit-Reset", "0")) or 0
                time.sleep(min(60, max(5, wait - time.time())) if wait else 15)
                continue
            raise SystemExit(f"{url}: HTTP {error.code} {error.reason}")
        except urllib.error.URLError as error:
            if attempt == 3:
                raise SystemExit(f"{url}: {error.reason}")
            time.sleep(3)
    raise SystemExit(f"{url}: gave up")


def fetch_window(repo, state, label, since):
    """Up to 1000 issues updated at or after `since`, oldest update first.

    Returns (issues, truncated). The endpoint has no `created` filter but does
    accept `since` on updated_at, so the walk advances along update time: each
    round is idempotent by issue number and the 1000 cap never bites.
    """
    collected = []
    for page in range(1, WINDOW_LIMIT // PER_PAGE + 1):
        params = {"state": state, "per_page": PER_PAGE, "page": page,
                  "sort": "updated", "direction": "asc"}
        if label:
            params["labels"] = label
        if since:
            params["since"] = since
        batch, headers = request(f"/repos/{repo}/issues", params)
        if not batch:
            break
        collected.extend(batch)
        print(f"  {repo} page {page}: {len(collected)} raw", file=sys.stderr, flush=True)
        if len(batch) < PER_PAGE:
            break
        remaining = headers.get("X-RateLimit-Remaining")
        if remaining is not None and int(remaining) < 3:
            raise SystemExit("rate limit nearly exhausted; set GITHUB_TOKEN and rerun")
        time.sleep(0.3)
    return collected, len(collected) >= WINDOW_LIMIT


def fetch(repo, state, label):
    out, seen, since = [], set(), None
    while True:
        batch, truncated = fetch_window(repo, state, label, since)
        fresh = [item for item in batch if item["number"] not in seen]
        seen.update(item["number"] for item in fresh)
        out.extend(fresh)
        if not truncated:
            return out
        newest = max(item["updated_at"] for item in batch)
        if since is not None and newest <= since:
            raise SystemExit(f"{repo}: {len(batch)} issues share one update timestamp; "
                             "narrow with --label")
        since = newest
        time.sleep(0.3)


def slim(items):
    keep = []
    for item in items:
        if "pull_request" in item:
            continue
        keep.append({
            "number": item["number"],
            "title": item["title"],
            "state": item["state"],
            "state_reason": item.get("state_reason"),
            "created_at": item["created_at"],
            "closed_at": item.get("closed_at"),
            "comments": item["comments"],
            "reactions": item["reactions"]["total_count"],
            "labels": [label["name"] for label in item["labels"]],
            "author": (item.get("user") or {}).get("login"),
            "body": item.get("body") or "",
        })
    keep.sort(key=lambda issue: issue["number"])
    return keep


def summarize(directory):
    for name in sorted(os.listdir(directory)):
        if not name.endswith(".json"):
            continue
        issues = json.load(open(os.path.join(directory, name)))
        reasons = {}
        labels = {}
        for issue in issues:
            reasons[issue["state_reason"]] = reasons.get(issue["state_reason"], 0) + 1
            for label in issue["labels"]:
                labels[label] = labels.get(label, 0) + 1
        top = sorted(labels.items(), key=lambda pair: -pair[1])[:12]
        print(f"{name}: {len(issues)} issues, closed_reason={reasons}")
        print("  labels: " + ", ".join(f"{name}={count}" for name, count in top))


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--repo", action="append", choices=REPOS, help="repeatable; default both")
    parser.add_argument("--state", default="closed", choices=("open", "closed", "all"))
    parser.add_argument("--label", help="e.g. enhancement; fewer issues, one request window")
    parser.add_argument("--out", default="/tmp/motty-competitor-issues", help="output directory")
    parser.add_argument("--summary", action="store_true", help="print counts for --out instead of fetching")
    args = parser.parse_args()

    if args.summary:
        summarize(args.out)
        return

    os.makedirs(args.out, exist_ok=True)
    for repo in args.repo or REPOS:
        print(f"{repo}: fetching {args.state} issues" + (f" labeled {args.label}" if args.label else ""),
              file=sys.stderr, flush=True)
        issues = slim(fetch(repo, args.state, args.label))
        path = os.path.join(args.out, repo.replace("/", "-").lower() + ".json")
        with open(path, "w") as handle:
            json.dump(issues, handle, ensure_ascii=False, indent=1)
        print(f"{repo}: {len(issues)} issues -> {path}", file=sys.stderr, flush=True)


if __name__ == "__main__":
    main()
