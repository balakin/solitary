#!/bin/sh
# What solitary patch runs in a cell's machine, as root: the upgrade
# unattended-upgrades would make in time, all of it and now. Embedded in the
# binary and run through limactl shell, with its output shown as it comes.
set -eu
export DEBIAN_FRONTEND=noninteractive

# A machine that has just booted is often already running apt's own daily
# job, which holds the lock for as long as it takes; waiting for it is the
# whole difference between patching and failing to.
lock="-o DPkg::Lock::Timeout=600"

apt-get $lock update
# full-upgrade rather than upgrade: a kernel fix arrives as a new package, and
# upgrade never installs one. A configuration file someone changed is kept,
# since nothing here can ask which one they meant. Services are restarted as
# Ubuntu's own needrestart decides, which never includes the user session the
# container runs in; what only a reboot applies, apt marks as needing one.
apt-get $lock -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold -y full-upgrade
