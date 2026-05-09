package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	getToken   string
	getJSON    bool
	getFormat  string
	getExport  bool
	getTimeout time.Duration
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get [secret-path]",
	Short: "Retrieve a secret value using an access token",
	Long: `Retrieve a secret value using an access token obtained from an approved request.
The token can be provided via --token flag or TRAVERSE_TOKEN environment variable.`,
	Example: `  # Get a secret with token
  traverse get prod/database/password --token <token>

  # Output as JSON for scripting
  traverse get prod/api/key --token <token> --json

  # Export as environment variables
  traverse get prod/config --token <token> --export

  # Use token from environment
  export TRAVERSE_TOKEN=<token>
  traverse get prod/database/password`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		secretPath := args[0]

		// Get token from flag or environment
		if getToken == "" {
			getToken = os.Getenv("TRAVERSE_TOKEN")
		}

		if getToken == "" {
			return fmt.Errorf("token is required (use --token or set TRAVERSE_TOKEN environment variable)")
		}

		// Retrieve secret
		secret, err := traverseClient.GetSecret(secretPath, getToken)
		if err != nil {
			return fmt.Errorf("failed to retrieve secret: %w", err)
		}

		// Output based on format
		switch getFormat {
		case "json":
			return outputJSON(secret)
		case "env":
			return outputEnv(secret, secretPath)
		case "export":
			return outputExport(secret, secretPath)
		default:
			return outputDefault(secret, secretPath)
		}
	},
}

func outputJSON(secret interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(secret)
}

func outputEnv(secret interface{}, path string) error {
	// Type assert to get the actual secret response
	s, ok := secret.(interface{ GetValues() map[string]string })
	if !ok {
		// Fallback for our actual type
		return outputDefault(secret, path)
	}

	values := s.GetValues()
	prefix := envPrefix(path)

	for key, value := range values {
		envVar := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(key))
		envVar = strings.ReplaceAll(envVar, "-", "_")
		fmt.Printf("%s=%s\n", envVar, value)
	}

	return nil
}

func outputExport(secret interface{}, path string) error {
	// Similar to env but with export prefix
	fmt.Printf("# Run this command to export secrets as environment variables:\n")
	fmt.Printf("# eval $(traverse get %s --token <token> --format export)\n\n", path)

	// Type assert
	s, ok := secret.(map[string]interface{})
	if !ok {
		return outputDefault(secret, path)
	}

	prefix := envPrefix(path)

	if values, ok := s["values"].(map[string]interface{}); ok {
		for key, value := range values {
			envVar := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(key))
			envVar = strings.ReplaceAll(envVar, "-", "_")
			fmt.Printf("export %s=\"%s\"\n", envVar, value)
		}
	}

	return nil
}

func outputDefault(secret interface{}, path string) error {
	// Handle both the actual type and generic map
	switch s := secret.(type) {
	case interface{ GetPath() string }:
		fmt.Printf("Secret: %s\n", s.GetPath())
	default:
		fmt.Printf("Secret: %s\n", path)
	}

	// Try to get values
	var values map[string]string
	switch s := secret.(type) {
	case interface{ GetValues() map[string]string }:
		values = s.GetValues()
	case map[string]interface{}:
		if v, ok := s["values"].(map[string]interface{}); ok {
			values = make(map[string]string)
			for key, val := range v {
				if str, ok := val.(string); ok {
					values[key] = str
				}
			}
		}
	}

	if len(values) == 0 {
		fmt.Println("(no values)")
		return nil
	}

	fmt.Printf("\nValues:\n")
	for key, value := range values {
		fmt.Printf("  %s: %s\n", key, value)
	}

	// Try to get access info
	switch s := secret.(type) {
	case interface{ GetAccess() interface{ GetExpiresAt() time.Time } }:
		access := s.GetAccess()
		fmt.Printf("\nAccess expires: %s\n", access.GetExpiresAt().Format(time.RFC3339))
	case map[string]interface{}:
		if access, ok := s["access"].(map[string]interface{}); ok {
			if expires, ok := access["expires_at"].(string); ok {
				fmt.Printf("\nAccess expires: %s\n", expires)
			}
		}
	}

	return nil
}

func envPrefix(path string) string {
	// Convert path to env-friendly prefix
	prefix := strings.ToUpper(path)
	prefix = strings.ReplaceAll(prefix, "/", "_")
	prefix = strings.ReplaceAll(prefix, "-", "_")
	prefix = strings.ReplaceAll(prefix, ".", "_")
	return prefix
}

func init() {
	RootCmd.AddCommand(getCmd)

	// Flags
	getCmd.Flags().StringVarP(&getToken, "token", "t", "", "Access token (or set TRAVERSE_TOKEN env var)")
	getCmd.Flags().StringVarP(&getFormat, "format", "f", "default", "Output format (default, json, env, export)")
	getCmd.Flags().BoolVar(&getJSON, "json", false, "Output as JSON (shorthand for --format json)")
	getCmd.Flags().BoolVarP(&getExport, "export", "e", false, "Output as export commands (shorthand for --format export)")

	// Set up shorthand aliases
	getCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if getJSON {
			getFormat = "json"
		}
		if getExport {
			getFormat = "export"
		}
		return nil
	}
}
