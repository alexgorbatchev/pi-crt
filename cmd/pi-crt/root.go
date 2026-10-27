package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"pi-crt/internal/library"
	"pi-crt/internal/media"
	"pi-crt/internal/player"
	"pi-crt/internal/schedule"
)

var version = "0.1.0-dev"

func newRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "pi-crt", Short: "Prepare and play offline CRT ambiance", Version: version, SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs, RunE: groupHelp}
	root.SetVersionTemplate("{{.Version}}\n")
	m := &cobra.Command{Use: "media", Short: "Prepare and inspect portable clip libraries", Args: cobra.NoArgs, RunE: groupHelp}
	opts := media.Options{}
	prepare := &cobra.Command{Use: "prepare MANIFEST OUTPUT", Short: "Trim sources and render static transitions on this computer", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		opts.Log = cmd.ErrOrStderr()
		if err := media.Prepare(cmd.Context(), args[0], args[1], opts); err != nil {
			return err
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), args[1])
		return err
	}}
	prepare.Flags().StringVar(&opts.Standard, "standard", "ntsc", "Output timing: ntsc or pal")
	prepare.Flags().IntVar(&opts.Noise, "noise", 4, "Luma noise strength (0..30)")
	check := &cobra.Command{Use: "check LIBRARY", Short: "Validate library metadata and media file availability", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		lib, err := library.Load(args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(lib)
	}}
	m.AddCommand(prepare, check)
	p := &cobra.Command{Use: "player", Short: "Play clips or preview their randomized order", Args: cobra.NoArgs, RunE: groupHelp}
	playOpts := player.Options{}
	run := &cobra.Command{Use: "run LIBRARY", Short: "Run one persistent VLC process until interrupted", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		playOpts.Log = cmd.ErrOrStderr()
		return player.Run(cmd.Context(), args[0], playOpts)
	}}
	run.Flags().StringVar(&playOpts.VLC, "vlc", "cvlc", "VLC executable path")
	run.Flags().IntVar(&playOpts.Port, "port", 42121, "Loopback VLC HTTP port (1024..65535)")
	run.Flags().IntVar(&playOpts.Recent, "recent", 5, "Recent clip exclusion count; clamped to library size minus one")
	run.Flags().IntVar(&playOpts.Count, "count", 0, "Stop after this many successful plays (0 means forever)")
	run.Flags().StringArrayVar(&playOpts.VLCArgs, "vlc-arg", nil, "Additional VLC argument; repeat for multiple arguments")
	var count, recent int
	order := &cobra.Command{Use: "order LIBRARY", Short: "Print a randomized playback sequence without launching VLC", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if count < 1 {
			return fmt.Errorf("count must be positive")
		}
		lib, err := library.Load(args[0])
		if err != nil {
			return err
		}
		bag, err := schedule.New(lib.Clips, recent)
		if err != nil {
			return err
		}
		for range count {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), bag.Next().ID); err != nil {
				return err
			}
		}
		return nil
	}}
	order.Flags().IntVar(&count, "count", 20, "Number of clip IDs to print")
	order.Flags().IntVar(&recent, "recent", 5, "Recent clip exclusion count")
	p.AddCommand(run, order)
	root.AddCommand(m, p, newSkillCommand())
	setupHelp(root)
	return root
}

func groupHelp(cmd *cobra.Command, _ []string) error { return cmd.Help() }
