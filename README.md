Offline Halloween ambiance for a Raspberry Pi 1 and a composite CRT: prepare clips on a Mac, copy the library to an SD card, and play in a shuffled loop.

# What It Does

- Cuts local videos or downloadable URLs into named snippets.
- Renders analog static between clips, subtle reception noise, and normalized audio.
- Plays every eligible clip once per round, with recent-history exclusion across rounds and a preference for changing source groups.
- Provides a Pi 1 ARMv6 binary and Raspberry Pi OS Lite installation, read-only configuration, and verification scripts.

# How It Works

- Describe clips and timestamps in a JSON manifest.
- Prepare a portable media folder on the editing computer with FFmpeg and yt-dlp.
- Run one persistent VLC process on the Pi, controlled locally by a small Go program.
- Keep playback history in RAM. The appliance configuration mounts the OS, boot partition, and media read-only, with bounded writable directories in RAM.

# How it Really Works

- Clips become 640×480, 4:3, H.264 Baseline level 3.0 with mono AAC. Wide sources are letterboxed. NTSC uses 30000/1001 fps; PAL uses 25 fps. Video bitrate is capped at 2 Mbit/s.
- Each clip begins with 0.4 seconds of static and quiet noise audio. The Pi does no effect rendering or downloading.
- The default excludes the five most recently selected clips, clamped to library size minus one. History survives reshuffles but resets at power-on. Clips sharing a `group` are separated when another eligible group exists. One clip necessarily repeats; very small libraries with maximum exclusion can settle into a fixed order. Lower `--recent` for more shuffle variety.
- VLC's HTTP interface binds only to loopback with a per-launch random password. Its playlist stays bounded to one item. Playback is sequential; short gaps between files remain possible. This is not a gapless compositor.
- Failed files are excluded until restart. Process supervision restarts the player after a crash. There is no desktop, login, or network-online dependency in the playback service.
- **Hardware qualification remains open:** actual Pi 1 decoding performance, DRM composite output, audio, cold-boot time, and repeated power cuts need testing on the board. The Mac tests do not establish those properties. A read-only configuration prevents normal filesystem writes; it is not a universal guarantee against electrical or SD-controller failures.

# Installation / Setup

The project is unreleased. Local deliverables are `bin/pi-crt` for this Mac,
`bin/pi-crt-linux-armv6` for the Pi, and `bin/pi-crt-pi1.tar.gz` with installation files.
There is no published release URL yet. Contributor build commands are in [AGENTS.md](AGENTS.md).
`bin/demo-preview.mp4` demonstrates the static transitions using generated test
signals; `bin/demo-library/` contains its three playable clips.

The preparation computer needs FFmpeg 8+ (including `libx264`, `dynaudnorm`, and
the scale filter's `reset_sar` option), FFprobe, and yt-dlp on PATH. Public URLs
must be supported by the installed yt-dlp version. Downloading is unnecessary
when using local files. Optional local playback requires VLC 3 with its Lua HTTP interface.

The Pi needs a 32 GB SD card and Raspberry Pi OS Lite **32-bit**. Installation
requires a writable, provisioned OS and temporary internet access for packages.
Follow [the bounded Pi setup and maintenance procedure](docs/pi-setup.md).
The firmware-only rainbow card cannot run this player; Linux and VLC replace that setup.

# Quick Start

1. Put source videos in `sources/`. Edit [examples/clips.json](examples/clips.json),
   replacing its sample path/timestamps with your own. `start` and `end` are seconds.
   A source may instead be a YouTube URL. Give snippets from the same film the same group.
2. Prepare a new library directory:

   ```sh
   ./bin/pi-crt media prepare examples/clips.json library
   ```

   Existing output directories are refused. Use a new directory when rebuilding;
   a failed render will not publish a half-finished library. PAL: add `--standard pal`.
3. Validate and inspect shuffle order:

   ```sh
   ./bin/pi-crt media check library
   ./bin/pi-crt player order library --count 30
   ```

4. Open a prepared MP4 in your Mac's video player to inspect the effects. Use
   `bin/demo-preview.mp4` for the supplied synthetic demonstration. Then follow
   the Pi setup procedure to install automatic looping playback.

The prepare command prints the output directory. Check prints JSON. Order prints
one clip ID per line. Preparation and playback diagnostics go to stderr.

# Options & Flags

| Command | Options |
| --- | --- |
| `media prepare MANIFEST OUTPUT` | `--standard ntsc` or `pal`; `--noise 4` (0–30) |
| `media check LIBRARY` | Metadata/file validation; does not decode every frame |
| `player order LIBRARY` | `--count 20`, `--recent 5` |
| `player run LIBRARY` | `--vlc cvlc`, `--port 42121`, `--recent 5`, `--count 0` (forever), repeatable `--vlc-arg` |
| `skill` | Embedded full CLI usage guide, available offline |
| `--help`, `--version` | Command help or raw version |

`AGENT=1`, `true`, or `yes` selects compact help. Data output stays the same.
Use `--vlc-arg=--option=value` for VLC options beginning with a dash.

# License

MIT © 2026 Alex Gorbatchev. See [LICENSE](LICENSE).
