# A throwaway xrdp server for tools/live/rdp_live.sh and tools/e2e/rdp_e2e.txt: user
# "motty" with password "motty-pw", a session that is one xterm.
FROM debian:bookworm-slim
# APT_PROXY (tools/live/rdp_live.sh passes DOCKER_BUILD_PROXY) reaches the package
# mirrors from behind a proxy; retries absorb a flaky one.
ARG APT_PROXY=
RUN printf 'Acquire::Retries "8";\nAcquire::http::Pipeline-Depth "0";\n' > /etc/apt/apt.conf.d/99retries && if [ -n "$APT_PROXY" ]; then printf 'Acquire::http::Proxy "%s";\n' "$APT_PROXY" > /etc/apt/apt.conf.d/99proxy; fi
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends xrdp xorgxrdp xterm x11-utils xdotool dbus-x11 ca-certificates procps && rm -rf /var/lib/apt/lists/* /etc/apt/apt.conf.d/99proxy
RUN useradd -m -s /bin/bash motty && echo 'motty:motty-pw' | chpasswd
COPY startwm.sh /etc/xrdp/startwm.sh
COPY start.sh /start.sh
RUN chmod +x /etc/xrdp/startwm.sh /start.sh
EXPOSE 3389
CMD ["/start.sh"]
