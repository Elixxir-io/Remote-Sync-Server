////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles command-line version functionality

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlab.com/xx_network/primitives/utils"
)

// Change this value to set the version for this build.
const currentVersion = "0.0.1"

// Version returns the current version and dependencies for this binary.
func Version() string {
	return fmt.Sprintf(
		"Haven Remote KV Server v%s -- %s\n\nDependencies:\n\n%s\n",
		SEMVER, GITVERSION, DEPENDENCIES)
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(generateCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version and dependency information for the binary",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(Version())
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates version and dependency information for the binary",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("genera\n")
		utils.GenerateVersionFile(currentVersion)
	},
}
