#!/bin/bash
# TigerVNC server on :1 (port 5901) with VNC authentication, plus an xterm to type into.
Xtigervnc :1 -rfbport 5901 -rfbauth /home/vnc/.vnc/passwd -SecurityTypes VncAuth -geometry 1024x768 -depth 24 -AlwaysShared -localhost no > /tmp/xvnc.log 2>&1 &
sleep 2
export DISPLAY=:1
xterm -geometry 80x24+40+40 -fa Monospace -fs 12 -bg black -fg white &
sleep infinity
