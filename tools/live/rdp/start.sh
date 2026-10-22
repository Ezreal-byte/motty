#!/bin/sh
rm -f /var/run/xrdp/*.pid /var/run/xrdp-sesman.pid
xrdp-sesman
exec xrdp --nodaemon
