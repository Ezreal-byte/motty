#!/bin/bash
cd /tmp
chmod +x /tmp/rfb.test
echo "=== :2 no security"
MOTTY_VNC_ADDR=127.0.0.1:5902 MOTTY_VNC_DISPLAY=:2 MOTTY_VNC_XEVLOG=/tmp/motty-vnc/xev2.log MOTTY_VNC_RESIZE=1 /tmp/rfb.test -test.run Live -test.v -test.timeout 90s 2>&1 
echo "=== :3 VNC authentication"
MOTTY_VNC_ADDR=127.0.0.1:5903 MOTTY_VNC_DISPLAY=:3 MOTTY_VNC_PASSWORD=motty-pw /tmp/rfb.test -test.run Live -test.v -test.timeout 90s 2>&1 
