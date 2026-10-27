package media

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const (
	staticPrefix   = 0.4
	staticDissolve = 0.2
)

func pictureFilter(cut Cut, rate string) string {
	crop := ""
	if r := cut.Crop; r != nil {
		crop = fmt.Sprintf("crop=%d:%d:%d:%d:exact=1,", r.Width, r.Height, r.X, r.Y)
	}
	return fmt.Sprintf("%sscale=320:240:force_original_aspect_ratio=increase:force_divisible_by=2:reset_sar=1,crop=320:240:x=(iw-ow)/2:y=(ih-oh)/2,setsar=1,fps=%s,format=yuv420p", crop, rate)
}

// Only the initial near-black run controls the overlay. This does not trim
// source time or replace intentional black frames later in the trailer.
func leadingBlack(ctx context.Context, src string, cut Cut, picture string) (float64, error) {
	d := cut.End - cut.Start
	filter := "setpts=PTS-STARTPTS," + picture + ",blackdetect=d=0:pix_th=0.02:pic_th=0.98,metadata=mode=print:file=-"
	b, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-v", "error", "-nostdin", "-ss", number(cut.Start), "-t", number(d), "-i", src, "-map", "0:v:0", "-vf", filter, "-an", "-f", "null", "-").Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return 0, fmt.Errorf("inspect opening frames: %w: %s", err, strings.TrimSpace(string(e.Stderr)))
		}
		return 0, fmt.Errorf("inspect opening frames: %w", err)
	}
	initial := false
	for line := range strings.SplitSeq(string(b), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || (key != "lavfi.black_start" && key != "lavfi.black_end") {
			continue
		}
		t, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, fmt.Errorf("parse opening frame timestamp: %w", err)
		}
		if key == "lavfi.black_start" {
			if t != 0 {
				return 0, nil
			}
			initial = true
		} else if initial {
			return min(t, d), nil
		}
	}
	if initial {
		return d, nil
	}
	return 0, nil
}

func tapeEffects(d, opening float64, rate string) string {
	start := staticPrefix + opening
	end := min(d+staticPrefix, start+staticDissolve)
	fadeVideo, fadeAudio := "", ""
	if opening < d {
		fadeVideo = fmt.Sprintf(",fade=t=out:st=%s:d=%s:alpha=1", number(start), number(staticDissolve))
		fadeAudio = fmt.Sprintf(",afade=t=out:st=%s:d=%s", number(start), number(staticDissolve))
	}
	// Two unequal burst periods keep the tape damage sparse. Six output rows and
	// coarse horizontal noise remain visible after 240p H.264 encoding.
	tracking := fmt.Sprintf("color=c=0xb8b8b8:s=80x2:r=%s:d=%s,format=yuv420p,noise=c0s=100:c0f=t+u:c0_seed=731,scale=320:6:flags=neighbor[tracking];[picture][tracking]overlay=x=0:y='2*mod(floor(t*18)*37,117)':enable='between(mod(t,5.3),1.4,1.72)+between(mod(t,8.7),3.1,3.28)':eof_action=pass:repeatlast=0,tpad=start_duration=%s[v];", rate, number(d), number(staticPrefix))
	static := fmt.Sprintf("color=c=gray:s=80x60:r=%s:d=%s,format=yuv420p,noise=c0s=100:c0f=t+u:c0_seed=417,scale=320:240:flags=neighbor,format=yuva420p%s[sv];[v][sv]overlay=eof_action=pass:repeatlast=0[outv];", rate, number(end), fadeVideo)
	audio := fmt.Sprintf("anoisesrc=r=48000:d=%s:a=0.06:seed=417%s,aformat=channel_layouts=mono[sa];[a][sa]amix=inputs=2:duration=first:normalize=0[outa]", number(end), fadeAudio)
	return tracking + static + audio
}
