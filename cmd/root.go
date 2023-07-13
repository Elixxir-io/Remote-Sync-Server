////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"fmt"
	"github.com/spf13/pflag"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"

	"gitlab.com/xx_network/primitives/utils"
)

// Execute initialises all config files, flags, and logging and then starts the
// server.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "remoteSyncServer",
	Short: "remoteSyncServer starts a secure remote sync server for Haven",
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(configFilePath)
		initLog(viper.GetString(logPathFlag), viper.GetUint(logLevelFlag))
		jww.INFO.Printf(Version())

		/* Do server stuff here. */
	},
}

var configFilePath string

const (
	logPathFlag  = "logPath"
	logLevelFlag = "logLevel"
)

// initConfig reads in config file from the file path.
func initConfig(filePath string) {
	// Use default config location if none is passed
	if filePath == "" {
		return
	}

	filePath, err := utils.ExpandPath(filePath)
	if err != nil {
		jww.FATAL.Panicf("Invalid config file path %q: %+v", filePath, err)
	}

	viper.SetConfigFile(filePath)

	viper.AutomaticEnv() // Read in environment variables that match

	// If a config file is found, read it in.
	if err = viper.ReadInConfig(); err != nil {
		jww.FATAL.Panicf("Invalid config file path %q: %+v", filePath, err)
	}
}

// initLog initialises the log to the specified log path filtered to the
// threshold. If the log path is "-" or "", it is printed to stdout.
func initLog(logPath string, threshold uint) {
	if logPath != "-" && logPath != "" {
		// Disable stdout output
		jww.SetStdoutOutput(io.Discard)

		// Use log file
		logOutput, err :=
			os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		jww.SetLogOutput(logOutput)
	}

	if threshold > 1 {
		jww.INFO.Printf("log level set to: TRACE")
		jww.SetStdoutThreshold(jww.LevelTrace)
		jww.SetLogThreshold(jww.LevelTrace)
		jww.SetFlags(log.LstdFlags | log.Lmicroseconds)
	} else if threshold == 1 {
		jww.INFO.Printf("log level set to: DEBUG")
		jww.SetStdoutThreshold(jww.LevelDebug)
		jww.SetLogThreshold(jww.LevelDebug)
		jww.SetFlags(log.LstdFlags | log.Lmicroseconds)
	} else {
		jww.INFO.Printf("log level set to: INFO")
		jww.SetStdoutThreshold(jww.LevelInfo)
		jww.SetLogThreshold(jww.LevelInfo)
	}
}

// init initializes all the flags for Cobra, which defines commands and flags.
func init() {
	rootCmd.PersistentFlags().StringVarP(&configFilePath, "config", "c", "",
		"File path to Custom configuration.")

	rootCmd.PersistentFlags().StringP(logPathFlag, "l", "",
		"File path to save log file to.")
	bindPFlag(rootCmd.PersistentFlags(), logPathFlag, rootCmd.Use)

	rootCmd.PersistentFlags().IntP(logLevelFlag, "v", 0,
		"Verbosity level for log printing (2+ = Trace, 1 = Debug, 0 = Info).")
	bindPFlag(rootCmd.PersistentFlags(), logLevelFlag, rootCmd.Use)
}

// bindPFlag binds the key to a pflag.Flag. Panics on error.
func bindPFlag(flagSet *pflag.FlagSet, key, use string) {
	err := viper.BindPFlag(key, flagSet.Lookup(key))
	if err != nil {
		jww.FATAL.Panicf(
			"Failed to bind key %q to a pflag on %s: %+v", key, use, err)
	}
}
