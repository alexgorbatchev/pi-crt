#!/bin/sh
# Run on a provisioned Raspberry Pi OS Lite 32-bit installation.
set -eu
if [ "$(uname -s)" != Linux ] || [ "$(id -u)" -ne 0 ]; then
    echo 'Run as root on the Raspberry Pi.' >&2
    exit 1
fi
if [ "$#" -ne 2 ]; then
    echo 'Usage: sudo sh scripts/install-pi.sh BINARY LIBRARY' >&2
    exit 1
fi
case "$(uname -m)" in armv6l) ;; *) echo 'This installer targets Pi 1 ARMv6 only.' >&2; exit 1 ;; esac
if [ ! -f /boot/firmware/config.txt ]; then
    echo 'Requires Raspberry Pi OS Lite with /boot/firmware (Bookworm or newer).' >&2
    exit 1
fi
case ",$(findmnt -n -o OPTIONS /)," in *,rw,*) ;; *) echo 'Root must be writable during installation.' >&2; exit 1 ;; esac
if [ -e /opt/pi-crt ]; then
    echo '/opt/pi-crt already exists; follow the maintenance procedure.' >&2
    exit 1
fi
binary=$(realpath "$1")
library=$(realpath "$2")
project=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
metadata=$("$binary" media check "$library")
apt-get update
apt-get install -y --no-install-recommends vlc-bin vlc-plugin-base jq
standard=$(printf '%s' "$metadata" | jq -er .standard)
backup="/var/backups/pi-crt-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$backup"
cp /boot/firmware/config.txt /boot/firmware/cmdline.txt /etc/fstab "$backup/"
if ! id pi-crt >/dev/null 2>&1; then
    useradd --system --home-dir /run/pi-crt --no-create-home --shell /usr/sbin/nologin --groups video,audio,render pi-crt
fi
install -d -m 0755 /opt/pi-crt
install -m 0755 "$binary" /opt/pi-crt/pi-crt
cp -R "$library" /opt/pi-crt/library
chown -R root:root /opt/pi-crt
chmod -R a+rX /opt/pi-crt
install -m 0644 "$project/deploy/pi-crt.service" /etc/systemd/system/pi-crt.service
install -d /etc/systemd/journald.conf.d
install -m 0644 "$project/deploy/journald.conf" /etc/systemd/journald.conf.d/pi-crt.conf
# Replace the desktop KMS overlay, preserving all unrelated boot settings.
sed -i '/^[[:space:]]*dtoverlay=vc4-\(f\?kms\)-v3d/d' /boot/firmware/config.txt
printf '\n[all]\ndtoverlay=vc4-kms-v3d,composite\ndtparam=audio=on\ndisable_splash=1\n' >> /boot/firmware/config.txt
if [ "$standard" = pal ]; then
    norm=PAL
    mode=720x576@50ie
else
    norm=NTSC
    mode=720x480@60ie
fi
# cmdline.txt remains one line. Remove previous display overrides first.
awk -v norm="$norm" -v mode="$mode" '{
    for (i=1;i<=NF;i++) if ($i !~ /^vc4.tv_norm=/ && $i !~ /^video=Composite-1:/ && $i !~ /^console=tty1$/) printf "%s ", $i;
    printf "vc4.tv_norm=%s video=Composite-1:%s quiet loglevel=3 vt.global_cursor_default=0\n", norm, mode;
}' /boot/firmware/cmdline.txt > "$backup/cmdline.new"
cat "$backup/cmdline.new" > /boot/firmware/cmdline.txt
systemctl set-default multi-user.target
systemctl disable NetworkManager-wait-online.service 2>/dev/null || true
systemctl mask getty@tty1.service
systemctl daemon-reload
systemctl enable pi-crt.service
printf 'Installed. Boot backups: %s\nReboot and verify picture/audio before running scripts/lock-pi.sh.\n' "$backup"
