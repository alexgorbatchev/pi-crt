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

- Clips become 320×240 progressive, square-pixel 4:3, H.264 Baseline level 3.0 with mono AAC. Sources scale proportionally to cover the screen, then crop equally around the center on both axes; widescreen footage loses its sides and portrait footage loses its top and bottom. No stretching or added black bars. NTSC uses 30000/1001 fps; PAL uses 25 fps. Video bitrate is capped at 2 Mbit/s. File resolution does not select the Pi's physical composite signal mode.
- Original source videos remain separate from prepared libraries. Local files stay unchanged; URL downloads and their provenance are retained under `sources/` beside the manifest and reused offline for later renders. YouTube editing masters remain 720p; preparation alone reduces them to 240p.
- Effects are baked into prepared files on the editing computer; the Pi does no effect rendering or downloading. Each clip starts with a 0.4-second static burst, including the first clip played. See [Effects and Timing](#effects-and-timing) for the processing and desktop-only CRT simulation.
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
the scale filter's `reset_sar` option) and FFprobe on PATH. First-time URL
downloads also require yt-dlp; URLs must be supported by its installed version.
Local files and retained URL downloads can be reprocessed without network access
or yt-dlp. Optional local playback requires VLC 3 with its Lua HTTP interface.

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

# Effects and Timing

`media prepare` renders the following effects into each output file. Changing
preparation settings requires preparing a new output library; existing libraries
remain unchanged.

| Effect | When it appears | Processing |
| --- | --- | --- |
| Light picture noise | Throughout the retained trailer footage | Animated, uniform luma noise; default `--noise 4`, range 0–30. `--noise 0` disables this layer without removing the static burst. |
| Blocky monochrome static | First 0.4 seconds of every prepared clip, including the first clip played | Strong animated noise generated at 80×60 and enlarged to 320×240 with nearest-neighbor scaling, producing 4×4 blocks. The final 100 ms fade toward black. |
| Quiet noise audio | During the 0.4-second static burst | Generated noise at amplitude 0.06, fading out over the final 100 ms. |
| Audio leveling | Throughout the retained trailer audio | Dynamic normalization with target peak 0.7, maximum gain 5, and soft compression factor 5. Audio is converted to 48 kHz mono; missing or short audio is padded with silence. |
| Audio fades | Beginning and end of the retained trailer audio | 20 ms fade-in and 30 ms fade-out. |
| CRT display simulation | During desktop playback with `just preview` or `just preview run` | CRT-Royale simulates scanlines, phosphor structure, and bloom in mpv. `just preview plain` omits this shader while retaining every baked-in effect. |

Preparation also applies each cut's optional source crop, scales proportionally
to cover 320×240, center-crops to fill 4:3, and converts to 30000/1001 fps (NTSC)
or 25 fps (PAL). Original source downloads remain unchanged. The trailer image
has no added video fade, color grading, screen curvature, grid, phosphor mask,
or scanlines. Physical CRT display characteristics are left to the display;
the CRT shader is for desktop preview only and never enters the prepared files
or the default Pi playback path. Reception noise and static simulate signal
interference separately from those display characteristics.

# Reprocessing Original Sources

Keep the downloaded masters and their provenance under ignored `sources/`.
The supplied horror manifest already points to the original 720p MP4 files in
`sources/horror-trailers/`. Crop rectangles and retained time ranges refer to
these originals, independently of output resolution and effects.

For a manifest containing HTTP(S) URLs, preparation saves each complete source
as `sources/<URL SHA-256>/source.mkv` beside that manifest, together with yt-dlp's
`source.info.json` provenance. For example, URL entries in `examples/clips.json`
use `examples/sources/`. Downloads are remuxed without re-encoding, probed, and
fully decoded before publication. YouTube downloads require genuine 720-line
video and audio, with H.264/AAC preferred; other URLs prefer formats up to 720p.
Later renders validate and reuse the retained file without contacting the URL.
Failed renders preserve completed downloads. Damaged retained sources cause an
error rather than being silently replaced.

1. Keep the original source files, saved editor projects, and provenance in place.
   Adjust the manifest's retained ranges or source-pixel crops as needed.
2. Render into a fresh output directory:

   ```sh
   ./bin/pi-crt media prepare examples/horror-trailers.json bin/library-240p
   ```

   `bin/` must exist and `bin/library-240p` must not exist. Each render starts
   from the source masters; it does not downscale an older prepared library or
   apply effects twice. Original files and existing libraries remain unchanged.
3. Validate and preview the result:

   ```sh
   ./bin/pi-crt media check bin/library-240p
   just preview run bin/library-240p
   ```

   `just preview plain bin/library-240p` plays the same files without the desktop
   CRT shader. Existing 640×480 libraries keep their old resolution and effects
   until replaced with a separately prepared 320×240 library.

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

# Non-destructive Trimming

Use [LosslessCut](https://github.com/mifi/lossless-cut) to choose the retained
portion of each trailer. The editor works from the original source. Its project
and CSV contain timestamps; applying them updates the manifest without modifying
the downloads or existing Pi files. Preparation encodes the selected range into
a new library, with the existing crop, static transition, and audio processing.

1. Install the editor once:

   ```sh
   just edit setup
   ```

   On macOS this installs LosslessCut through Homebrew. On other systems install
   LosslessCut yourself and put its executable on PATH.
2. Open a trailer by manifest ID:

   ```sh
   just edit run psycho-1960
   ```

   This creates a native `psycho-1960-proj.llc` beside the downloaded source,
   initialized with the manifest's current range. Existing projects are preserved
   when reopened. The default manifest is `examples/horror-trailers.json`; pass
   another manifest as the last argument. Editing requires a local source and
   FFprobe, without downloads.
3. Seek to the first frame to retain and press `I`; seek to the end and press
   `O`. Space plays/pauses. Keep one segment spanning the desired trailer,
   excluding the intro and outro. Export the **project segments as CSV (seconds)**
   to `bin/edits/psycho-1960.csv`. This workflow uses the timestamp CSV rather
   than LosslessCut's video export button. Its keyframe export limitations do
   not affect the project's normal re-encoded preparation.
4. Apply the decision to the manifest:

   ```sh
   just edit apply psycho-1960 bin/edits/psycho-1960.csv
   git diff -- examples/horror-trailers.json
   ```

   CSV has one headerless `start,end,label` row in source seconds; for example,
   `8.25,370.5,Retained trailer`. Labels and extra tag columns are ignored.
   Empty start means zero; empty end is a marker and is rejected. Multiple rows,
   timecode-formatted exports, nonfinite or negative times, ranges under 0.5
   seconds, and ends beyond the source duration are rejected. Failed imports
   preserve the manifest. Successful imports atomically replace it while retaining
   permissions, IDs, groups, sources, crops, and other clips; symlinks are refused.
5. Repeat for other trailers, then render and preview a new library:

   ```sh
   just prepare examples/horror-trailers.json library-trimmed
   just preview run library-trimmed
   ```

   Use an output directory that does not already exist. Trim timestamps are
   relative to the original source, without the prepared clip's 0.4-second static
   prefix. `git diff` shows each decision, and editing the timestamps again lets
   you restore footage. Saved editor projects are independent of the manifest;
   to restart from changed manifest values, rename the old `.llc` project before
   reopening. No intro/outro boundaries are selected automatically.

# Options & Flags

| Command | Options |
| --- | --- |
| `media prepare MANIFEST OUTPUT` | `--standard ntsc` or `pal`; `--noise 4` (0–30) |
| `media check LIBRARY` | Metadata/file validation; does not decode every frame |
| `media edit MANIFEST CLIP` | Create/reopen native LosslessCut project; prints its path |
| `media trim MANIFEST CLIP CSV` | Import one retained range from CSV (seconds); atomically updates MANIFEST |
| `player order LIBRARY` | `--count 20`, `--recent 5` |
| `player run LIBRARY` | `--vlc cvlc`, `--port 42121`, `--recent 5`, `--count 0` (forever), repeatable `--vlc-arg` |
| `skill` | Embedded full CLI usage guide, available offline |
| `--help`, `--version` | Command help or raw version |

`AGENT=1`, `true`, or `yes` selects compact help. Data output stays the same.
Use `--vlc-arg=--option=value` for VLC options beginning with a dash.

# License

MIT © 2026 Alex Gorbatchev. See [LICENSE](LICENSE).
