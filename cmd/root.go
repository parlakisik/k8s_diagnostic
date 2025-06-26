package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "k8s-diagnostic",
	Short: "A CLI tool for Kubernetes diagnostic testing",
	Long: `k8s-diagnostic is a command line tool for testing network connectivity
within Kubernetes clusters.

This tool works with any Kubernetes cluster:
- Use with test clusters created by build_test_k8s.sh script
- Use with real clusters by providing kubeconfig file

The tool provides various commands to test network connectivity,
DNS resolution, and other networking aspects within Kubernetes clusters.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔗 k8s-diagnostic - Kubernetes Diagnostic Testing Tool")
		fmt.Println("")
		fmt.Println("Available commands:")
		fmt.Println("  test    - Run diagnostic tests")
		fmt.Println("")
		fmt.Println("Use --help for more information about available commands")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.k8s-diagnostic.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().String("kubeconfig", "", "path to kubeconfig file (uses default kubectl config if not specified)")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".k8s-diagnostic" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".k8s-diagnostic")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
