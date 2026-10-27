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

- Clips become 640×480, square-pixel 4:3, H.264 Baseline level 3.0 with mono AAC. Sources scale proportionally to cover the screen, then crop at the center; widescreen footage loses its sides and portrait footage loses its top and bottom. No stretching or added black bars. NTSC uses 30000/1001 fps; PAL uses 25 fps. Video bitrate is capped at 2 Mbit/s.
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
   The five downloaded horror trailers use [examples/horror-trailers.json](examples/horror-trailers.json)
   and local source files under `sources/horror-trailers/`; these videos are not included in Git.
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

4. Preview the entire library locally using the procedure below. Then follow
   the Pi setup procedure to install automatic looping playback.

The prepare command prints the output directory. Check prints JSON. Order prints
one clip ID per line. Preparation and playback diagnostics go to stderr.

For sources with black borders already baked into the video, add a `crop`
rectangle to the cut before preparing. For example, `"crop": {"x": 0, "y": 86,
"width": 1280, "height": 548}` selects the active picture of a 1280×720 source.
Coordinates are source pixels; the rectangle must stay within the source.
Inspect bright scenes to distinguish borders from dark footage. The selected
picture then scales and center-crops to fill 4:3, which can trim titles and
subjects near the edges. Rebuild older letterboxed libraries into a new directory
to apply this framing; fullscreen playback alone cannot remove bars baked into files.

# Local Sequence Preview

The preview uses [mpv](https://mpv.io/) with the
[CRT-Royale port for mpv](https://github.com/hhirtz/mpv-retro-shaders).
It simulates a CRT display using a GPU shader, including scanlines, phosphor
structure, and bloom. This is a display approximation; physical composite
output, overscan, Pi decoding, audio latency, and VLC's file-change timing still
require hardware testing. The shader runs only on the preparation computer.

1. From the repository root, install the preview dependencies and cache the shader:

   ```sh
   just preview setup
   ```

   This installs mpv through Homebrew when missing and downloads a pinned,
   SHA-256-verified shader into ignored `.cache/`. Go, just, and jq must already
   be installed. On systems without Homebrew, install mpv through your package
   manager first. Subsequent previews work offline using the cached shader.
2. Use the existing prepared `library/`, or prepare sources into a new directory:

   ```sh
   just prepare examples/horror-trailers.json library-new
   just media-check library-new
   ```

   Preparation requires the local source downloads described above. Existing
   output directories are refused; do not rerun preparation against `library/`.
3. Preview every clip once, in a freshly shuffled order:

   ```sh
   just preview
   ```

   The default library is `library/`. Its five horror trailers run for about
   14 minutes 20 seconds. The preview plays the actual 4:3 prepared files,
   including their baked-in static, reception noise, and normalized audio.
   Space pauses, `f` toggles fullscreen, `<` / `>` change clips, arrow keys seek,
   and `q` quits. The window closes when the sequence finishes.
4. Compare the plain picture or specify a library, selection count, recent-history
   window, and optional mpv flags:

   ```sh
   just preview plain
   just preview run library-new 0 5 --fullscreen
   just preview run library 20 2
   ```

   Count `0` selects exactly one round. A positive count runs that many selections
   through the same scheduler as the Pi, including exclusions across rounds.
   Each invocation generates a new shuffle. For an exact visual comparison,
   reuse the generated playlist directly:

   ```sh
   mpv --no-config --playlist=bin/preview/sequence.m3u8
   ```

The playlist contains absolute file URLs and is local to this computer. Generate
it without opening a player using `just preview playlist`. Decode a complete
round without a window or audio using `just preview test`. `just --list
--list-submodules` lists all commands, including `prepare`, `media-check`,
`order`, and `play`. `just play` runs the actual VLC-controlled player and
requires `cvlc` on PATH; on macOS pass `--vlc /Applications/VLC.app/Contents/MacOS/VLC`.

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
