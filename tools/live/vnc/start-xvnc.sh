#!/bin/bash
# Start throwaway VNC servers for the rfb live tests: :2 with no security on 5902,
# :3 with VNC authentication (password "motty-pw") on 5903.
set -u
mkdir -p /tmp/motty-vnc
cd /tmp/motty-vnc
cp /tmp/motty-passwd /tmp/motty-vnc/passwd
chmod 600 /tmp/motty-vnc/passwd
for spec in "2 5902 None" "3 5903 VncAuth"; do
  set -- $spec
  extra=""
  [ "$3" = VncAuth ] && extra="-rfbauth /tmp/motty-vnc/passwd"
  setsid nohup Xvnc :$1 -rfbport $2 -SecurityTypes $3 $extra -geometry 1024x768 -depth 24 -interface 127.0.0.1 -disableBasicAuth -noWebsocket -AlwaysShared -AcceptPointerEvents 1 -AcceptKeyEvents 1 -AcceptCutText 1 -SendCutText 1 > /tmp/motty-vnc/xvnc$1.log 2>&1 < /dev/null &
done
sleep 2
export DISPLAY=:2
setsid nohup xev -display :2 -geometry 400x300+0+0 -event keyboard -event button > /tmp/motty-vnc/xev2.log 2>&1 < /dev/null &
setsid nohup xmessage -display :3 -geometry +100+100 "hello from motty live test" > /tmp/motty-vnc/xmessage.log 2>&1 < /dev/null &
sleep 1
ps aux | grep -E "Xvnc :|xev|xmessage" | grep -v grep | cut -c1-150
ss -ltn | grep -E "590[23]"
cat /tmp/motty-vnc/xvnc2.log | tail -5
