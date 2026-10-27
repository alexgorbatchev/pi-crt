#!/bin/sh
# Make normal appliance boots read-only. Does not reboot automatically.
set -eu
if [ "$(uname -s)" != Linux ] || [ "$(id -u)" -ne 0 ] || [ ! -f /opt/pi-crt/pi-crt ]; then
    echo 'Run as root on an installed pi-crt appliance.' >&2
    exit 1
fi
case ",$(findmnt -n -o OPTIONS /)," in *,rw,*) ;; *) echo 'Already read-only; use the maintenance procedure.' >&2; exit 1 ;; esac
if grep -q '# pi-crt read-only' /etc/fstab; then
    echo 'Read-only configuration already exists; reboot and verify it.' >&2
    exit 1
fi
if grep -q 'overlayroot=' /boot/firmware/cmdline.txt; then
    echo 'Disable the existing overlay filesystem and reboot before locking.' >&2
    exit 1
fi
if awk '$1 !~ /^#/ && ($2 == "/var" || $2 == "/tmp") {found=1} END {exit !found}' /etc/fstab; then
    echo 'Existing /var or /tmp fstab mounts require manual adaptation before locking.' >&2
    exit 1
fi
backup="/root/pi-crt-before-lock-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$backup"
cp /etc/fstab /boot/firmware/cmdline.txt "$backup/"
swapoff -a
systemctl mask dphys-swapfile.service
mkdir -p /etc/rpi/swap.conf.d
printf '[Main]\nMechanism=none\n' > /etc/rpi/swap.conf.d/99-pi-crt.conf
# Read-only /var and an offline appliance have no use for update jobs or a
# network wait. Maintenance boot instructions restore access explicitly.
systemctl mask apt-daily.timer apt-daily-upgrade.timer systemd-random-seed.service
systemctl disable NetworkManager.service 2>/dev/null || true
awk '
    /^[[:space:]]*#/ || NF < 4 { print; next }
    $3 == "swap" { print "# pi-crt disabled swap: " $0; next }
    $2 == "/" || $2 == "/boot/firmware" {
        n=split($4,a,","); options="";
        for(i=1;i<=n;i++) if(a[i]!="rw" && a[i]!="ro") options=options a[i] ",";
        $4=options "ro";
    }
    { print }
    END {
        print "# pi-crt read-only runtime mounts";
        print "tmpfs /var tmpfs defaults,nosuid,nodev,size=32M,mode=0755 0 0";
        print "tmpfs /tmp tmpfs defaults,nosuid,nodev,size=16M,mode=1777 0 0";
    }
' /etc/fstab > "$backup/fstab.locked"
cat "$backup/fstab.locked" > /etc/fstab
awk '{ for(i=1;i<=NF;i++) if($i!="rw" && $i!="ro") printf "%s ",$i; print "ro" }' /boot/firmware/cmdline.txt > "$backup/cmdline.locked"
cat "$backup/cmdline.locked" > /boot/firmware/cmdline.txt
sync
printf 'Read-only boot configured. Backup: %s\nReboot normally, then run scripts/verify-pi.sh before pull-plug testing.\n' "$backup"
