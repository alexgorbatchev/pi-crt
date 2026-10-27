---
created_on: 2026-10-26 22:51
last_modified: 2026-10-26 23:30
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

## Setup and constraints

- Go 1.26.2+, just, golangci-lint 2.13.2. Keep downloaded dependencies in normal shared caches; use .tmp/go for task compilation scratch.
- No SD card is needed for development. Actual composite, audio, cold boot, and power-cut behavior must be measured on the Pi before claiming hardware readiness.
- Playback history, VLC profile, and logs must remain in RAM in appliance mode. The read-only root approach supports Pi 1 memory variants without an overlay filesystem.
- Media preparation may write files and download sources; the Pi playback path must not transcode, download, or wait for network availability.
- Follow official current Raspberry Pi KMS composite documentation. Firmware-only sdtv settings do not configure the Linux KMS pipeline.
- Small libraries cannot provide both a long exclusion window and unconstrained randomness. Maintain fairness and exclusion across round boundaries; source grouping is a soft preference.

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
