package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string

	RootCmd = &cobra.Command{
		Use:   "nervegate",
		Short: "NerveGate: The Sub-Millisecond Intelligent Gateway & Model Orchestrator for Linux",
		Long: `NerveGate is a high-performance, Linux-native LLM proxy gateway designed to route
requests with microsecond-level latency overhead (< 50µs). It acts as nature's nervous
system for AI agents, dynamically evaluating work type, task complexity, and criticality.`,
	}
)

func Execute(version, commit, buildDate string) {
	RootCmd.AddCommand(newServeCmd())
	RootCmd.AddCommand(newVersionCmd(version, commit, buildDate))
	RootCmd.AddCommand(newBenchmarkCmd())

	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/nervegate/config.yaml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home + "/.config/nervegate")
			viper.AddConfigPath(".")
			viper.SetConfigName("config")
			viper.SetConfigType("yaml")
		}
	}
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}
