#!/bin/bash
# Live tests of internal/rfb against real VNC servers (see docs/remote-desktop.md).
#
#   tools/live/vnc_live.sh docker   TigerVNC in a throwaway container (motty-e2e-vnc,
#                              port 5901 on loopback, password motty-pw); the
#                              container and its image are removed afterwards
#   tools/live/vnc_live.sh linux    KasmVNC Xvnc on the shared Linux test machine
#                              (../viva/tools/testenv): :2 without security, :3
#                              with VNC authentication
#
# The Docker build needs network; DOCKER_BUILD_PROXY sets the build's proxy.
set -eu
cd "$(dirname "$0")/../.."
here=tools/live/vnc
case "${1:-}" in
docker)
	trap 'docker rm -f motty-e2e-vnc >/dev/null 2>&1; docker rmi motty-e2e-vnc:latest >/dev/null 2>&1' EXIT
	cp "$here/container-start.sh" "$here/start.sh"
	docker build --build-arg https_proxy="${DOCKER_BUILD_PROXY:-http://host.docker.internal:7897}" -t motty-e2e-vnc:latest "$here"
	rm -f "$here/start.sh"
	docker run -d --name motty-e2e-vnc -p 127.0.0.1:5901:5901 motty-e2e-vnc:latest
	sleep 5
	MOTTY_VNC_ADDR=127.0.0.1:5901 MOTTY_VNC_PASSWORD=motty-pw go test ./internal/rfb -run Live -v -count=1
	;;
linux)
	env=../viva/tools/testenv
	GOOS=linux GOARCH=amd64 go test -c -o /tmp/rfb.test ./internal/rfb
	$env/upload.sh linux /tmp/rfb.test /tmp/rfb.test
	$env/upload.sh linux "$here/passwd.bin" /tmp/motty-passwd
	for f in start-xvnc.sh stop-xvnc.sh run-live.sh; do $env/upload.sh linux "$here/$f" "/tmp/$f"; done
	$env/exec.sh linux 'bash /tmp/stop-xvnc.sh; bash /tmp/start-xvnc.sh >/dev/null 2>&1; setsid nohup bash /tmp/run-live.sh > /tmp/motty-live.log 2>&1 < /dev/null &'
	echo "running on the test machine (read /tmp/motty-live.log there); afterwards clean up with:"
	echo "  $env/exec.sh linux 'bash /tmp/stop-xvnc.sh; rm -f /tmp/rfb.test /tmp/motty-live.log /tmp/*xvnc.sh /tmp/run-live.sh /tmp/motty-passwd'"
	;;
*)
	sed -n 2,11p "$0"
	exit 2
	;;
esac
