FROM debian:bookworm-slim
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends tigervnc-standalone-server tigervnc-tools xterm x11-utils xdotool xauth ca-certificates && rm -rf /var/lib/apt/lists/*
RUN useradd -m vnc && mkdir -p /home/vnc/.vnc && printf 'motty-pw\n' | vncpasswd -f > /home/vnc/.vnc/passwd && chmod 600 /home/vnc/.vnc/passwd && chown -R vnc /home/vnc
USER vnc
ENV HOME=/home/vnc
COPY start.sh /home/vnc/start.sh
CMD ["bash", "/home/vnc/start.sh"]
