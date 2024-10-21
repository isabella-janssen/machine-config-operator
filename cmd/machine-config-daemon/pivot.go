package main

import (
	"flag"
	"fmt"
	"os"

	// Enable sha256 in container image references
	_ "crypto/sha256"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var pivotCmd = &cobra.Command{
	Use:                   "pivot",
	DisableFlagsInUseLine: true,
	Short:                 "Allows moving from one OSTree deployment to another",
	Args:                  cobra.MaximumNArgs(1),
	Run:                   Execute,
}

// init executes upon import
func init() {
	rootCmd.AddCommand(pivotCmd)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
}

// Execute runs the command
func Execute(_ *cobra.Command, _ []string) {
	fmt.Println(`
	Testing!!
	`)
	os.Exit(1)
}
