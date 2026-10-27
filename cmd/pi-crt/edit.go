package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"pi-crt/internal/media"
)

func newEditCommand() *cobra.Command {
	return &cobra.Command{Use: "edit MANIFEST CLIP", Short: "Create or locate a non-destructive LosslessCut project", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		path, err := media.EditProject(cmd.Context(), args[0], args[1])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
		return err
	}}
}

func newTrimCommand() *cobra.Command {
	return &cobra.Command{Use: "trim MANIFEST CLIP CSV", Short: "Apply a retained range from LosslessCut CSV to the manifest", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		if err := media.Trim(cmd.Context(), args[0], args[1], args[2]); err != nil {
			return err
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), args[0])
		return err
	}}
}
