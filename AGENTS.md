---
created_on: 2026-10-26 22:51
last_modified: 2026-10-27 09:20
status: current
---

# pi-crt

Prepare clips on the Mac; run an offline composite-video appliance on a Pi 1 with a 32 GB SD card. Work on software and install assets does not require an attached card.

## Commands

- `just build`: local binary under bin/.
- `just build-pi`: static Linux ARMv6 binary; GOARM=6 is essential for Pi 1.
- `just test`: offline unit tests with race detection.
- `just integration`: real FFmpeg rendering and decoding tests; requires FFmpeg 8+, FFprobe, yt-dlp. Set `PI_CRT_VLC` to a VLC executable to include headless player tests.
- `just check`: module hygiene, vet, offline tests, golangci-lint, shell syntax, justfile formatting.
- `just package`: local Pi installation archive and SHA-256 checksum; publishes nothing.
- `just run ...` / `just run-ai ...`: human/agent CLI.
- `just prepare [MANIFEST OUTPUT]`, `just media-check [LIBRARY]`, `just order [LIBRARY ...FLAGS]`, `just play [LIBRARY ...FLAGS]`: build and invoke the corresponding CLI commands; preparation refuses existing output directories.
- `just preview setup`: install missing mpv through Homebrew and cache the pinned, checksum-verified CRT-Royale shader; requires jq. Other systems must install mpv separately.
- `just preview` / `just preview plain`: play a full shuffled round from `library/` with/without the CRT shader. Custom arguments: `just preview run LIBRARY COUNT RECENT ...MPV_FLAGS`; count 0 means one round. `just preview playlist` generates a local M3U8; `just preview test [LIBRARY]` decodes a complete round with real mpv and no display/audio.
- `just edit setup`, `just edit run CLIP [MANIFEST]`, `just edit apply CLIP CSV [MANIFEST]`: install/open LosslessCut and import one retained range from CSV (seconds). Default manifest: `examples/horror-trailers.json`. Source-side `.llc` projects preserve editor state; ignored `bin/edits/` holds CSV exports.

## Setup and constraints

- Go 1.26.2+, just, golangci-lint 2.13.2. Keep downloaded dependencies in normal shared caches; use .tmp/go for task compilation scratch.
- No SD card is needed for development. Actual composite, audio, cold boot, and power-cut behavior must be measured on the Pi before claiming hardware readiness.
- Playback history, VLC profile, and logs must remain in RAM in appliance mode. The read-only root approach supports Pi 1 memory variants without an overlay filesystem.
- Media preparation may write files and download sources; the Pi playback path must not transcode, download, or wait for network availability.
- Follow official current Raspberry Pi KMS composite documentation. Firmware-only sdtv settings do not configure the Linux KMS pipeline.
- Small libraries cannot provide both a long exclusion window and unconstrained randomness. Maintain fairness and exclusion across round boundaries; source grouping is a soft preference.

## Media quality

- Download YouTube source trailers at 720p, one standard resolution tier above the project's 640×480 CRT playback files. Prefer H.264 video with AAC audio in MP4; preserve source aspect ratio and frame rate. Do not download 1080p/4K or upscale lower-resolution uploads to meet this requirement; find a genuine 720p upload instead.
- Keep source downloads and their YouTube provenance under ignored `sources/`. Check each download with FFprobe for 720-line video and an audio stream, then decode the complete file with FFmpeg to detect corruption.
- Prepare Pi playback separately through `media prepare`: 640×480, square pixels, 4:3 filling the entire screen, H.264 Baseline level 3.0, maximum 2 Mbit/s video, mono AAC, and the library's NTSC/PAL frame rate. Preserve proportions by scaling to cover and center-cropping; never stretch or add letterboxing/pillarboxing. Source downloads are editing inputs, not Pi-ready playback files.
- Remove baked-in borders with each cut's optional `crop` rectangle in source pixels before fitting to 4:3. Inspect representative bright scenes to choose the active picture; darkness in horror footage makes automatic black-border detection unreliable. Validate crops against source dimensions and verify framing with decoded-frame tests. Perform all cropping during preparation, not on the Pi.
- Keep intro/outro trimming non-destructive: preserve original downloads, store retained source-time ranges in the versioned manifest, and prepare a new output library. Use LosslessCut's native projects and CSV (seconds), not a custom video editor or intermediate video exports. Import only the one retained range for the named clip; preserve all other decisions and source crops. Existing editor projects take precedence over manifest changes when reopening; rename an old project to reinitialize it. Never guess trailer boundaries automatically.
- Keep local full-sequence previews in the `preview/justfile` module. Reuse `media check` and `player order` for validation and scheduling; play prepared files without re-encoding. CRT simulation uses mpv's gpu-next and the pinned CRT-Royale port from hhirtz/mpv-retro-shaders. On macOS use `--macos-render-timer=system`; the default CVDisplayLink timer can stall when no screen callback is available. Cache shaders in ignored `.cache/` and playlists/screenshots in ignored `bin/preview/`. Do not confuse desktop shader rendering with physical CRT or Pi qualification.

## Conventions and verification

- Prefer standard library mechanisms. Cobra and cobra-help-tree/v2 are the only direct Go dependencies besides pflag in interface-drift tests.
- Verify CLI changes against the embedded cmd/pi-crt/SKILL.md in the same change. Include generated help/completion commands, argument semantics, flags, defaults, accepted values, environment, output, failures, and side effects. Update its last_modified field.
- Retain byte-for-byte embedded skill tests in both modes, output error tests, help coverage, and live command-tree drift tests.
- Runtime behavior changes require corresponding behavioral test changes. Require 90% aggregate Go statement coverage with integration tests; scripts/ is excluded. Do not test static config constants as a substitute for behavior.
- Test real FFmpeg/VLC when available; use httptest for HTTP protocol edge cases. Do not introduce production mocks or fake playback.
- Before code review, apply the code-review-baseline skill. Use codegraph if this project has an index.
- Automatically record new durable project instructions here; check with the user when they conflict with existing instructions.
- Never publish releases, tags, packages, or deployments without explicit authorization. Do not commit generated media, downloaded sources, or binaries.
- GoReleaser publishing is intentionally disabled while this is a local repository without a release destination. Local packaging is available through `just package`.
- Private internal domains defined in the global agent instructions must never be committed/pushed to github.com, including private repositories. Inspect destination and changes before commits/pushes.
