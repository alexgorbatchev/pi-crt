---
created_on: 2026-10-26 23:30
last_modified: 2026-10-26 23:30
status: current
---

# Pi 1 installation and qualification

These procedures are prepared for hardware testing, not yet verified on a physical
Pi. Use a Raspberry Pi 1, its 32 GB SD card, stable USB power, yellow RCA video,
and separate analog audio if wanted. The original yellow RCA socket carries video
only. A keyboard or serial console is needed for maintenance after networking is
disabled. Use another virtual terminal (Ctrl-Alt-F2) for local login; tty1 belongs
to playback.

## Install and establish playback

1. Flash Raspberry Pi OS Lite **32-bit**, Bookworm or newer, onto the intended
   32 GB card. This replaces the firmware-only test card contents. Complete the
   initial OS provisioning and first boot while power is stable. Configure a local
   user and temporary Ethernet/SSH access. The yellow-RCA Pi 1 has no built-in Wi-Fi.
2. Copy and extract `bin/pi-crt-pi1.tar.gz` on the Pi. Copy a prepared library from
   the Mac separately. The archive contains the controller and deployment files;
   it is not a complete OS image. From the extracted directory run:

   ```sh
   sudo sh scripts/install-pi.sh ./pi-crt-linux-armv6 ./library
   sudo reboot
   ```

   The installer refuses non-ARMv6 systems, an existing installation, or a read-only
   root. It validates the library before installing packages. It copies media to
   `/opt/pi-crt/library`, creates an unprivileged service account, installs VLC,
   configures KMS composite for the library's standard, enables boot playback,
   and disables the network-online wait and tty1 login. It records boot-file
   backups under `/var/backups/pi-crt-TIMESTAMP`. Partial installation errors
   require inspection before rerunning; the script does not erase an existing
   installation to recover automatically.
3. Confirm moving picture, correct aspect ratio, and analog audio. Keep HDMI
   disconnected during composite qualification. Inspect failures with:

   ```sh
   systemctl status pi-crt.service
   journalctl -u pi-crt.service -b --no-pager
   aplay -L
   ls /sys/class/drm/
   ```

   The service requests `Composite-1` and `sysdefault:CARD=Headphones`. Confirm those
   names on the actual image. A different ALSA name requires adjusting the service
   argument. The installed VLC must expose the DRM video-output module and Lua HTTP
   interface. If it reports a missing module or software decode cannot keep up,
   stop qualification and resolve the Pi's VLC/driver configuration first.
   Do not call dropped frames or a restarted player successful playback.
4. Play enough clips to cross multiple shuffle boundaries. Verify that corrupt
   clips get skipped and the service restarts after a terminated player process.
   Measure CPU/memory use during playback. A 256 MB board must be tested explicitly;
   cross-compiling an ARMv6 binary does not establish that VLC fits its memory budget.

## Enable read-only appliance boots

1. While playback works and the root is still writable, run:

   ```sh
   sudo sh scripts/lock-pi.sh
   sudo reboot
   ```

   This makes `/` and `/boot/firmware` read-only, disables swap and background
   package jobs, disables NetworkManager, and adds tmpfs mounts for `/var` (32 MB)
   and `/tmp` (16 MB). These are ceilings, not reserved allocations. `/run` is
   already RAM-backed on Raspberry Pi OS. Journals are volatile with an 8 MB limit.
   Playback profile/cache live under `/run/pi-crt`. `/var`'s underlying package
   database is preserved on disk but hidden until a maintenance boot.
   Backups are saved under `/root/pi-crt-before-lock-TIMESTAMP`.
2. From a local console, run:

   ```sh
   sudo sh scripts/verify-pi.sh
   ```

   Do not proceed if it reports writable block-backed storage, swap, missing RAM
   mounts, or an inactive service. Also confirm visible moving picture and audio:
   an active process alone is insufficient. Custom partitions, swap generators,
   or local OS changes may require additional configuration.
3. Measure **power-on to first moving picture** over at least five cold boots.
   Use `systemd-analyze time` and `systemd-analyze critical-chain pi-crt.service`
   to diagnose OS/service delays; they do not measure firmware or first-frame time.
   The provided service has no desktop, login, or network-online dependency.
   First OS provisioning boots are not representative of appliance boot time.
4. After verification, perform repeated power cuts at different points in
   playback and confirm every subsequent boot. Normal operation writes no SD
   files. Do not pull power during initial provisioning, installation, maintenance,
   or filesystem changes. Read-only operation reduces corruption exposure; it
   cannot guarantee the behavior of every SD controller or power supply.

## Update clips or software

1. At a local console, stop playback and remount the underlying filesystems:

   ```sh
   sudo systemctl stop pi-crt.service
   sudo mount -o remount,rw /
   sudo mount -o remount,rw /boot/firmware
   sudo ls -d /root/pi-crt-before-lock-*
   ```

2. Choose the backup path printed by the locking script. Restore its `fstab` to
   `/etc/fstab` and `cmdline.txt` to `/boot/firmware/cmdline.txt`. Use the actual
   timestamped path. Then enable temporary networking and reboot:

   ```sh
   sudo systemctl enable NetworkManager.service
   sudo reboot
   ```

   This reboot is required before package work: simply remounting root does not
   reveal the package database hidden by the RAM-backed `/var`. Swap and apt timers
   can remain disabled during maintenance.
3. Stop playback. Copy a new prepared library to `/opt/pi-crt/library-new` and run
   `/opt/pi-crt/pi-crt media check /opt/pi-crt/library-new`. Rename the existing
   library to `library-old`, then rename `library-new` to `library`. Keep ownership
   root:root and files readable by the pi-crt account. For software updates,
   replace `/opt/pi-crt/pi-crt` with the ARMv6 binary while the service is stopped.
   Avoid changing NTSC/PAL as part of a media-only update; changing the standard
   also requires updating boot display settings and rebooting.
4. Restart playback and qualify the changes. Run the locking procedure again,
   reboot, and verify read-only operation before returning to pull-plug use.

## Technical references

- [Raspberry Pi OS: VLC, DRM and ALSA playback](https://www.raspberrypi.com/documentation/computers/os.html#play-audio-and-video)
- [Raspberry Pi KMS composite configuration](https://www.raspberrypi.com/documentation/computers/config_txt.html#composite-video-mode)
- [Linux connector mode command-line syntax](https://docs.kernel.org/fb/modedb.html)
- [systemd service execution and filesystem protection](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html)
- [Raspberry Pi swap configuration](https://github.com/raspberrypi/rpi-swap)
- [Go ARM target selection](https://go.dev/wiki/GoArm)
