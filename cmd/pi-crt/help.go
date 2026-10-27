package main

import (
	"fmt"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
	"pi-crt/internal/agent"
)

var techCatalog = cobrahelptree.TechCatalog{
	"pi-crt": {
		Summary:     "Offline composite-video ambiance player for Raspberry Pi 1",
		Description: "Offline composite-video ambiance player for Raspberry Pi 1 - command-line interface.",
	},
	"pi-crt media prepare": {
		Summary:  "Render a new 320x240 clip library; retain URL downloads beside the manifest for offline reprocessing",
		Args:     []cobrahelptree.ArgSpec{{Name: "MANIFEST", Description: "JSON clip manifest"}, {Name: "OUTPUT", Description: "New library directory"}},
		Metadata: map[string]string{"writes": "OUTPUT, temporary sibling directory, and sources/ beside MANIFEST for URL downloads", "network": "uncached HTTP(S) sources only"},
	},
	"pi-crt media check":  {Args: []cobrahelptree.ArgSpec{{Name: "LIBRARY", Description: "Prepared media directory"}}},
	"pi-crt media edit":   {Args: []cobrahelptree.ArgSpec{{Name: "MANIFEST", Description: "JSON clip manifest"}, {Name: "CLIP", Description: "Clip ID"}}, Metadata: map[string]string{"writes": "CLIP-proj.llc beside the local source; existing projects preserved", "network": "none"}},
	"pi-crt media trim":   {Args: []cobrahelptree.ArgSpec{{Name: "MANIFEST", Description: "JSON clip manifest to update"}, {Name: "CLIP", Description: "Clip ID"}, {Name: "CSV", Description: "LosslessCut CSV (seconds), one retained segment"}}, Metadata: map[string]string{"writes": "atomic replacement of MANIFEST", "network": "none"}},
	"pi-crt player order": {Args: []cobrahelptree.ArgSpec{{Name: "LIBRARY", Description: "Prepared media directory"}}},
	"pi-crt player run":   {Args: []cobrahelptree.ArgSpec{{Name: "LIBRARY", Description: "Prepared media directory"}}, Metadata: map[string]string{"state": "RAM only; VLC profile paths controlled by environment", "network": "loopback VLC HTTP"}},
}

func setupHelp(cmd *cobra.Command) {
	err := cobrahelptree.SetupWithOptions(cmd, cobrahelptree.HelpOptions{
		Catalog: techCatalog,
		Tree:    cobrahelptree.TreeOptions{HideGeneratedCommands: true},
	})
	if err != nil {
		panic(fmt.Sprintf("configure CLI help: %v", err))
	}
	help := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if agent.IsAgentMode() {
			if _, err := fmt.Fprintln(c.OutOrStdout(), "ALERT: Agents must read `AGENT=1 pi-crt skill` before using this tool."); err != nil {
				c.PrintErrln(err)
				return
			}
		}
		help(c, args)
	})
}
