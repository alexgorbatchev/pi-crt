---
name: pi-crt
description: Use when operating the pi-crt CLI.
author: Alex Gorbatchev
metadata:
  created_on: 2026-10-26 22:51
  last_modified: 2026-10-27 09:20
  status: current
---

## Execution rules

Read this skill before operational commands. Set `AGENT=1` for compact help.
Values `1`, `true`, and `yes` enable agent mode, ignoring case and surrounding
whitespace. Human mode is the default. Errors exit nonzero and print `ERR:`
in agent mode or `[ERROR]` otherwise. Data output is identical in both modes.
Ctrl-C and SIGTERM cancel active work and exit successfully.
Media preparation uses FFmpeg and FFprobe from PATH, plus yt-dlp for HTTP(S)
sources. Only downloads need network access; all other operations are offline.
VLC is required for playback. HOME and XDG variables affect VLC's own profile;
the supplied Pi service redirects these into RAM. The controller stores no
persistent runtime state. Preparation uses a scratch directory beside OUTPUT.

## `pi-crt`

Print root help with no command. Accept no positional arguments. All commands
accept `--help` / `-h`. Agent help starts with an alert to read this skill.

| Flag | Short | Type | Default | Behavior |
| --- | --- | --- | --- | --- |
| `--help` | `-h` | `bool` | `false` | Print command help. |
| `--version` | `-v` | `bool` | `false` | Print raw version and newline. |

## `pi-crt media`

Print media command help. Accept no positional arguments or specific flags.

## `pi-crt media prepare MANIFEST OUTPUT`

Read a JSON manifest with a `clips` array. Each cut has a unique `id` matching
`[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}`, a local or HTTP(S) `source`, numeric `start`
and `end` seconds, and optional `group`. Start defaults to zero. End is required,
must be within source duration, and must exceed start by at least 0.5 seconds.
Local paths are relative to MANIFEST. Group defaults to source; give snippets
from the same film matching groups even when their source paths differ.
Reject unknown JSON fields, duplicate IDs, missing tools and invalid cuts.

Each cut may include `crop: {"x": 0, "y": 86, "width": 1280, "height": 548}`
to remove borders baked into the source. Coordinates and dimensions are integers
in source pixels. Require nonnegative x/y, positive width/height, and a rectangle
wholly within the source. Apply this crop before scaling. Without it, use the
whole source frame. Scale proportionally to cover 640x480, account for source
pixel aspect ratio, then center-crop to square-pixel 4:3. Fill the entire screen
without stretching or adding borders. Widescreen sources lose their sides;
portrait sources lose their top and bottom. Existing prepared libraries must be
prepared again into a new directory to receive the framing change.

OUTPUT must not exist and its parent must exist. Render in a temporary sibling,
then rename into OUTPUT after success. Failed preparation removes scratch.
An abrupt kill may leave `.pi-crt-prepare-*` scratch in the parent.
Downloads are temporary and reused within a single preparation run.
yt-dlp ignores user config, downloads one item and remuxes to MKV. Prefer formats
up to 720p; accept unknown source resolution when the extractor cannot report it.
Publicly accessible sources require no credentials; downloader failures are errors.
Prepared files are 640x480 H.264 baseline level 3.0, yuv420p, mono AAC 48 kHz,
cropped to fill 4:3, with dynamic audio normalization, optional luma noise, and 0.4
seconds of static at the beginning. NTSC is 30000/1001 fps; PAL is 25 fps.
Write `clips/*.mp4` and `library.json`. Print OUTPUT to stdout; progress and
external-tool diagnostics go to stderr.

| Flag | Short | Type | Default | Behavior |
| --- | --- | --- | --- | --- |
| `--standard` | — | `string` | `ntsc` | Accept ntsc or pal. |
| `--noise` | — | `int` | `4` | Luma noise strength, 0 through 30. |

## `pi-crt media edit MANIFEST CLIP`

Create a native LosslessCut v2 project for CLIP, identified by its manifest ID.
Require a valid manifest, a local source file, and FFprobe. Resolve local sources
relative to MANIFEST. Probe the source and reject cuts beyond its duration.
Write `CLIP-proj.llc` beside the source with its current start/end range and
source basename, then print the absolute project path. Preserve an existing
regular project byte-for-byte; reject nonregular projects. Do not launch the
editor, download, transcode, or modify videos. Open the resulting file in
LosslessCut, mark the retained range, and export CSV (seconds).

## `pi-crt media trim MANIFEST CLIP CSV`

Apply one retained range from LosslessCut's CSV (seconds) export. No flags.
Require one headerless `start,end[,label,...tags]` row. Times are source seconds;
an empty start means zero. Ignore labels/tags. Reject markers (empty end),
multiple segments, human-readable timecodes, invalid/nonfinite/negative times,
ranges shorter than 0.5 seconds, unknown IDs, and ends beyond source duration
(0.05s tolerance). Require FFprobe and a local source; no network access.

Atomically replace MANIFEST in place, retaining its permissions, other clips,
groups, sources, and crops. Refuse manifest symlinks. Print MANIFEST after
success. The source video and prepared libraries remain untouched. Rebuild
into a new output directory to apply the trim, then preview that library.
Example: `pi-crt media trim examples/horror-trailers.json psycho-1960 cuts.csv`.

## `pi-crt media check LIBRARY`

Validate library.json and nonempty regular media files within LIBRARY.
Reject escaping paths and symlinks. Print the library as one JSON object.
This checks structure and file presence, not whether every frame decodes.
Accept no specific flags; write no files and require no external tools.

## `pi-crt player`

Print player command help. Accept no positional arguments or specific flags.

## `pi-crt player run LIBRARY`

Load the library once, then start VLC with its HTTP interface bound to
127.0.0.1 and a random password. One persistent process plays one file at a
time; transitions are baked into files. Playback may have a short gap.
Shuffle without replacement; preserve recent exclusion across rounds and
prefer a different source group among eligible clips. Clamp recent to clip
count minus one; a single clip necessarily repeats. History resets on restart.
Small libraries with maximum exclusion can repeat a fixed sequence.
Skip failed clips until restart; fail when all clips fail. Bound VLC startup
to 20 seconds, clip startup to 15 seconds and playback to duration plus 30 seconds.
Log plays, skips and VLC diagnostics to stderr. Cancellation reaps VLC.
Require decoded-video and displayed-picture counters before counting a play as
successful. The supplied Linux service uses DRM output. For headless testing,
pass `--vlc-arg=--vout=vdummy --vlc-arg=--aout=adummy --vlc-arg=--avcodec-hw=none`.
Inspect individual prepared files in a desktop video player for a visual preview.

| Flag | Short | Type | Default | Behavior |
| --- | --- | --- | --- | --- |
| `--vlc` | — | `string` | `cvlc` | VLC executable name or path. |
| `--port` | — | `int` | `42121` | Loopback HTTP port, 1024 through 65535. |
| `--recent` | — | `int` | `5` | Nonnegative recent exclusion count. |
| `--count` | — | `int` | `0` | Successful plays before exit; zero means forever. |
| `--vlc-arg` | — | `stringArray` | `[]` | Repeatable VLC args; use --vlc-arg=--vout=dummy syntax. |

## `pi-crt player order LIBRARY`

Print one clip ID per line using the playback shuffle. Require no VLC or network
and write no files. Every invocation starts with fresh random state.

| Flag | Short | Type | Default | Behavior |
| --- | --- | --- | --- | --- |
| `--count` | — | `int` | `20` | Positive number of IDs to print. |
| `--recent` | — | `int` | `5` | Nonnegative recent exclusion, clamped to size minus one. |

## `pi-crt skill`

Print this embedded file verbatim in either mode from any directory.
Accept no positional arguments or specific flags. Output-write failures are errors.

## `pi-crt help [command]`

Print root or command help, e.g. `pi-crt help media prepare`. No specific flags.

## `pi-crt completion`

Print shell completion help. Accept no positional arguments or specific flags.
The commands below print scripts to stdout and require no network or credentials.

## `pi-crt completion bash`

Print a Bash script. Load with `source <(pi-crt completion bash)` with bash-completion.
Accept no positional arguments.

| Flag | Short | Type | Default | Behavior |
| --- | --- | --- | --- | --- |
| `--no-descriptions` | — | `bool` | `false` | Omit descriptions. |

## `pi-crt completion zsh`

Print a Zsh script. Run `autoload -U compinit; compinit`, then
`source <(pi-crt completion zsh)`. Accept no positional arguments.

| Flag | Short | Type | Default | Behavior |
| --- | --- | --- | --- | --- |
| `--no-descriptions` | — | `bool` | `false` | Omit descriptions. |

## `pi-crt completion fish`

Print a Fish script. Load with `pi-crt completion fish | source`.
Accept no positional arguments.

| Flag | Short | Type | Default | Behavior |
| --- | --- | --- | --- | --- |
| `--no-descriptions` | — | `bool` | `false` | Omit descriptions. |

## `pi-crt completion powershell`

Print a PowerShell script. Load with
`pi-crt completion powershell | Out-String | Invoke-Expression`.
Accept no positional arguments.

| Flag | Short | Type | Default | Behavior |
| --- | --- | --- | --- | --- |
| `--no-descriptions` | — | `bool` | `false` | Omit descriptions. |

## Workflow

```sh
export AGENT=1
pi-crt skill
pi-crt media prepare clips.json library
pi-crt media check library
pi-crt player order library --count 30
pi-crt player run library
```
