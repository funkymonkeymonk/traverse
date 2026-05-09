package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/funkymonkeymonk/traverse/pkg/client"
	"github.com/spf13/cobra"
)

var (
	requestSecretPath string
	requestReason     string
	requestClientID   string
	requestDuration   string
	requestPoll       bool
	requestPollInterval time.Duration
	requestTimeout    time.Duration
)

// requestCmd represents the request command
var requestCmd = &cobra.Command{
	Use:   "request [secret-path]",
	Short: "Submit a request for secret access",
	Long: `Submit a request to access a secret. The request will be sent to approvers
who can approve or deny it. You can optionally poll for the result.`,
	Example: `  # Request access to a secret
  traverse request prod/database/password --reason "Need for deployment"

  # Request with polling for automatic retrieval when approved
  traverse request prod/api/key --reason "Bug fix" --poll

  # Request with custom timeout and poll interval
  traverse request staging/db --reason "Testing" --poll --poll-interval 5s --timeout 10m`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get secret path from args or flag
		if len(args) > 0 {
			requestSecretPath = args[0]
		}

		if requestSecretPath == "" {
			return fmt.Errorf("secret path is required")
		}

		if requestReason == "" {
			return fmt.Errorf("reason is required (use --reason)")
		}

		// Validate reason length
		if len(requestReason) < 10 {
			return fmt.Errorf("reason must be at least 10 characters long")
		}

		// Create request
		req := client.CreateRequestRequest{
			SecretPath:        requestSecretPath,
			Reason:            requestReason,
			ClientID:          requestClientID,
			RequestedDuration: requestDuration,
		}

		fmt.Printf("Submitting request for: %s\n", requestSecretPath)
		fmt.Printf("Reason: %s\n", requestReason)

		resp, err := traverseClient.CreateRequest(req)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		fmt.Printf("\nRequest submitted successfully!\n")
		fmt.Printf("Request ID: %s\n", resp.RequestID)
		fmt.Printf("Status: %s\n", resp.Status)
		fmt.Printf("Expires at: %s\n", resp.ExpiresAt.Format(time.RFC3339))

		// Poll if requested
		if requestPoll {
			fmt.Printf("\nPolling for approval...\n")
			return pollForApproval(resp.RequestID)
		}

		fmt.Printf("\nCheck status with: traverse status %s\n", resp.RequestID)
		return nil
	},
}

func pollForApproval(requestID string) error {
	startTime := time.Now()
	attempts := 0

	for {
		// Check timeout
		if time.Since(startTime) > requestTimeout {
			return fmt.Errorf("timeout waiting for approval after %v", requestTimeout)
		}

		status, err := traverseClient.GetStatus(requestID)
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}

		attempts++

		switch status.Status {
		case "approved":
			fmt.Printf("\n✓ Request approved!\n")
			fmt.Printf("Token: %s\n", status.Token)
			if status.TokenExpiresAt != nil {
				fmt.Printf("Token expires at: %s\n", status.TokenExpiresAt.Format(time.RFC3339))
			}

			// Automatically retrieve the secret if we have a token
			if status.Token != "" {
				fmt.Printf("\nRetrieving secret...\n")
				return retrieveSecret(status.SecretPath, status.Token)
			}
			return nil

		case "denied":
			fmt.Printf("\n✗ Request denied\n")
			if len(status.DeniedBy) > 0 {
				fmt.Printf("Denied by: %s\n", status.DeniedBy[0].Identity)
				if status.DeniedBy[0].Reason != "" {
					fmt.Printf("Reason: %s\n", status.DeniedBy[0].Reason)
				}
			}
			return fmt.Errorf("request was denied")

		case "expired":
			return fmt.Errorf("request expired before approval")

		case "pending":
			// Show progress
			fmt.Printf("\r[%d] Waiting for approval... (elapsed: %v)",
				attempts, time.Since(startTime).Round(time.Second))
			time.Sleep(requestPollInterval)

		default:
			fmt.Printf("\r[%d] Unknown status: %s", attempts, status.Status)
			time.Sleep(requestPollInterval)
		}
	}
}

func retrieveSecret(path, token string) error {
	secret, err := traverseClient.GetSecret(path, token)
	if err != nil {
		return fmt.Errorf("failed to retrieve secret: %w", err)
	}

	fmt.Printf("\nSecret: %s\n", secret.Path)
	fmt.Printf("Provider: %s\n", secret.Provider)
	fmt.Printf("\nValues:\n")

	for key, value := range secret.Values {
		fmt.Printf("  %s: %s\n", key, value)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(requestCmd)

	// Flags
	requestCmd.Flags().StringVarP(&requestReason, "reason", "r", "", "Reason for requesting access (required, min 10 chars)")
	requestCmd.Flags().StringVar(&requestClientID, "client-id", "", "Client identifier")
	requestCmd.Flags().StringVar(&requestDuration, "duration", "1h", "Requested access duration")
	requestCmd.Flags().BoolVarP(&requestPoll, "poll", "p", false, "Poll for approval and auto-retrieve secret")
	requestCmd.Flags().DurationVar(&requestPollInterval, "poll-interval", 5*time.Second, "Interval between status checks")
	requestCmd.Flags().DurationVar(&requestTimeout, "timeout", 10*time.Minute, "Maximum time to wait for approval")

	// Mark required flags
	requestCmd.MarkFlagRequired("reason")
}

// formatJSON outputs data as formatted JSON
func formatJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// parseMetadata parses a slice of key=value strings into a map
func parseMetadata(pairs []string) map[string]string {
	result := make(map[string]string)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
