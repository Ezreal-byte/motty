#!/bin/bash
# Stop the throwaway servers (never :1, the VM's own desktop).
pkill -x xev; pkill -x xmessage
pkill -f 'Xvnc :[23] -rfbport'
sleep 1
rm -rf /tmp/motty-vnc /tmp/.X2-lock /tmp/.X3-lock /tmp/.X11-unix/X2 /tmp/.X11-unix/X3
ps aux | grep -E "Xvnc :[23]|xev|xmessage" | grep -v grep | wc -l
