#!/bin/sh
# The session of the test server: a terminal that fills the screen.
xsetroot -solid '#203040' 2>/dev/null
exec xterm -geometry 120x40+0+0 -fa Monospace -fs 14 -bg black -fg white
