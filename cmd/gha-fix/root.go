package main

import (
	"log/slog"
	"os"

	"github.com/phsym/console-slog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "gha-fix",
	Short: "Fix GitHub Actions workflow files",
	Long: `A utility tool for automating GitHub Actions workflow security and maintainability improvements.
gha-fix provides various commands to automatically fix common issues in GitHub Actions workflow files.
`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	logLevel := new(slog.LevelVar)
	slog.SetDefault(slog.New(console.NewHandler(os.Stderr, &console.HandlerOptions{
		Level:      logLevel,
		NoColor:    !term.IsTerminal(int(os.Stderr.Fd())),
		TimeFormat: "2006-01-02 15:04:05.000",
	})))

	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./gha-fix.yaml)")

	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "set log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringSlice("ignore-dirs", []string{".git", "node_modules", "dist", "out", "vendor", ".idea", ".vscode", "bin", "build", "tmp", "coverage", ".cache", "__pycache__"}, "Comma-separated list of directory names to ignore when searching for workflow files")
	cobra.OnInitialize(func() {
		level := viper.GetString("log-level")
		switch level {
		case "debug":
			logLevel.Set(slog.LevelDebug)
		case "", "info": // default to info if no level is specified
			logLevel.Set(slog.LevelInfo)
		case "warn":
			logLevel.Set(slog.LevelWarn)
		case "error":
			logLevel.Set(slog.LevelError)
		default:
			logLevel.Set(slog.LevelInfo)
			slog.Warn("invalid log level specified, using 'info'", "specified", level)
		}
	})

	// Bind the ignore-dirs flag explicitly to ensure it's available globally
	cobra.CheckErr(viper.BindPFlag("ignore-dirs", rootCmd.PersistentFlags().Lookup("ignore-dirs")))

	// Bind all persistent flags
	cobra.CheckErr(viper.BindPFlags(rootCmd.PersistentFlags()))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("gha-fix")
	}

	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err == nil {
		slog.Info("using config file", "path", viper.ConfigFileUsed())
	}
}
