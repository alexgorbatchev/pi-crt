#!/bin/sh
# A failure means the appliance is not ready for pull-plug testing.
set -eu
if [ "$(uname -s)" != Linux ]; then echo 'Run on the Pi.' >&2; exit 1; fi
for path in / /boot/firmware /opt/pi-crt/library; do
    case ",$(findmnt -n -o OPTIONS -T "$path")," in
        *,ro,*) ;;
        *) printf 'FAIL: %s is writable\n' "$path" >&2; exit 1 ;;
    esac
done
for path in /var /tmp /run; do
    if [ "$(findmnt -n -o FSTYPE -T "$path")" != tmpfs ]; then
        printf 'FAIL: %s is not RAM-backed\n' "$path" >&2; exit 1
    fi
done
if [ -n "$(swapon --show --noheadings)" ]; then
    echo 'FAIL: active swap' >&2; exit 1
fi
# Any additional writable block-backed mount also breaks the appliance contract.
if findmnt -rn -o SOURCE,OPTIONS | awk '$1 ~ /^\/dev\// && $2 ~ /(^|,)rw(,|$)/ {found=1; print > "/dev/stderr"} END {exit !found}'; then
    echo 'FAIL: writable block-backed mounts remain' >&2; exit 1
fi
systemctl is-active --quiet pi-crt.service
echo 'Read-only mounts, RAM runtime directories, no swap, and active player verified.'
echo 'Confirm moving picture and audio, then measure boot time and perform repeated power-cut tests.'
