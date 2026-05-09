// Package cli provides the traverse CLI commands
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/funkymonkeymonk/traverse/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"

	// Config file path
	cfgFile string

	// Global flags
	serverURL string
	apiKey    string

	// Global client (initialized after config loading)
	traverseClient *client.Client
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "traverse",
	Short: "Traverse CLI - MFA secrets proxy with approval workflows",
	Long: `Traverse provides secure access to secrets with multi-factor authentication
and approval workflows. Use this CLI to request secrets, approve requests,
and manage access.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip client initialization for certain commands
		if cmd.Name() == "version" || cmd.Name() == "completion" {
			return nil
		}

		// Initialize client
		if err := initClient(); err != nil {
			return err
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.traverserc)")
	RootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "", "Traverse server URL")
	RootCmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")

	// Bind flags to viper
	viper.BindPFlag("server", RootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("api_key", RootCmd.PersistentFlags().Lookup("api-key"))

	// Environment variable bindings
	viper.BindEnv("server", "TRAVERSE_SERVER")
	viper.BindEnv("api_key", "TRAVERSE_API_KEY")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not find home directory: %v\n", err)
		} else {
			// Search config in home directory with name ".traverserc"
			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")
			viper.SetConfigName(".traverserc")
		}
	}

	// Read in environment variables that match
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		// Don't print this during completion generation
		if os.Getenv("_CLI_ZSH_COMPLETION") == "" && os.Getenv("_CLI_BASH_COMPLETION") == "" {
			fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
		}
	}
}

// initClient creates the API client from configuration
func initClient() error {
	url := viper.GetString("server")
	if url == "" {
		url = "http://localhost:8080"
	}

	key := viper.GetString("api_key")

	var err error
	traverseClient, err = client.NewClient(url, key)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	return nil
}

// GetClient returns the initialized client
func GetClient() *client.Client {
	return traverseClient
}

// GetDefaultConfigPath returns the default config file path
func GetDefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".traverserc")
}
