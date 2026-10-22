#!/bin/sh
# What solitary inspect asks a cell's machine. Embedded in the binary and run
# through limactl shell as the machine's user, so it reads and never changes:
# no sudo, no apt-get update. Every line is key=value on one line, and a fact
# the machine cannot answer is left out rather than guessed.

. /etc/os-release && echo "release=$PRETTY_NAME"
echo "kernel=$(uname -r)"

if [ -e /run/reboot-required ]; then echo "reboot=yes"; else echo "reboot=no"; fi

# apt-check prints "updates;security" on stderr, counted against the package
# lists as they were last refreshed — which is why that time is reported too.
if [ -x /usr/lib/update-notifier/apt-check ]; then
	echo "updates=$(/usr/lib/update-notifier/apt-check 2>&1 >/dev/null)"
fi
stamp=/var/lib/apt/periodic/update-success-stamp
if [ -e "$stamp" ]; then
	echo "lists=$(stat -c %Y "$stamp")"
else
	newest=$(ls -t /var/lib/apt/lists/*InRelease 2>/dev/null | head -n 1)
	[ -n "$newest" ] && echo "lists=$(stat -c %Y "$newest")"
fi
echo "unattended=$(systemctl is-enabled unattended-upgrades.service 2>/dev/null)"

echo "disk=$(df -B1 --output=used,size / | tail -n 1)"

# What the cell's isolation is made of, and what verifies the TLS it speaks.
for package in podman crun util-linux nftables dnsmasq-base wireguard-tools systemd openssl ca-certificates; do
	version=$(dpkg-query -W -f='${Version}' "$package" 2>/dev/null) && echo "package.$package=$version"
done
exit 0
