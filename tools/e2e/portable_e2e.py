#!/usr/bin/env python3
"""End to end for a portable copy: a motty-data folder beside the program is the
data directory, and the saved-passwords vault is kept by a master password alone.

    cd tools/e2esshd && go build -o /tmp/e2esshd . && /tmp/e2esshd 2222 &
    go build -tags vivahead -o /tmp/motty-head ./cmd/motty
    python3 tools/e2e/portable_e2e.py

The program is copied beside a motty-data folder in a temporary directory and
started with no MOTTY_DATA_DIR, as a user would after unpacking the zip. It
checks that: everything is written inside motty-data and nothing under HOME; the
password dialog says a master password is needed rather than offering a box that
is not there; and after a restart the saved password is used only once the master
password has been typed. The headless build keeps secrets in memory, so what this
shows is the data directory and the vault file, not the operating system's store.
"""
import json, os, shutil, subprocess, sys, tempfile

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import headless_drive

PORT = os.environ.get("MOTTY_E2E_PORT", "2222")
BINARY = os.environ.get("MOTTY_HEAD", "/tmp/motty-head")
root = tempfile.mkdtemp()
program = os.path.join(root, "app")
data = os.path.join(program, "motty-data")
home = os.path.join(root, "home")
os.makedirs(data)
os.makedirs(home)
shutil.copy(BINARY, os.path.join(program, "motty"))
failures = []


def check(name, ok, detail=""):
    print(("PASS " if ok else "FAIL ") + name + ("" if ok else f"  {detail}"))
    if not ok:
        failures.append(name)


def run(commands):
    script = os.path.join(root, "script.txt")
    with open(script, "w") as handle:
        handle.write(commands)
    env = dict(os.environ, VIVA_HEADLESS="1", HOME=home, MOTTY_LANG="en")
    env.pop("MOTTY_DATA_DIR", None)
    proc = subprocess.Popen([os.path.join(program, "motty")], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, env=env, text=True, bufsize=1, cwd=root)
    try:
        return headless_drive.drive(proc, script)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()


# First run: the password dialog cannot offer to save until a master password is set.
check("first run reaches the password dialog", run(f"""
click "Search"
type tester@127.0.0.1:{PORT}
click "Connect"
waitfor "Trust" 3000
click "Trust and Save"
waitfor "set a master password in Settings" 3000
""") == 0)

# Set the master password in Settings, then save a password with it.
check("master password set and password saved", run(f"""
click "Settings"
click "Security"
click "Set"
type correct horse
key tab
type correct horse
click "Save"
click "Hosts"
click "Search"
type tester@127.0.0.1:{PORT}
click "Connect"
waitfor "Password" 8000
type secret
click "Save password"
click "Connect"
waitfor "tester@e2e" 8000
""") == 0)

vault_path = os.path.join(data, "vault.json")
check("vault file sits in motty-data", os.path.exists(vault_path))
if os.path.exists(vault_path):
    vault = json.load(open(vault_path))
    check("vault is wrapped by a master password", vault.get("kdf", {}).get("name") == "argon2id" and len(vault["records"]) == 1)
check("settings and known hosts sit in motty-data", all(os.path.exists(os.path.join(data, name)) for name in ("config.db", "known_hosts")))
check("nothing was written under HOME", os.listdir(home) == [], str(os.listdir(home)))

# Second run: the tab is restored and connecting asks for the master password,
# then signs in with the saved password without a password dialog.
check("restart: unlock, then the saved password signs in", run("""
click "Connect"
waitfor "Master password" 8000
type correct horse
key enter
waitfor "tester@e2e" 8000
""") == 0)

shutil.rmtree(root, ignore_errors=True)
sys.exit(1 if failures else 0)
