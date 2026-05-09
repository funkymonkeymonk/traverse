package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	statusWatch   bool
	statusJSON    bool
	statusInterval time.Duration
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [request-id]",
	Short: "Check the status of a secret request",
	Long: `Check the current status of a secret request. You can watch for changes
or output the status as JSON for scripting.`,
	Example: `  # Check status of a request
  traverse status req_12345

  # Watch for status changes
  traverse status req_12345 --watch

  # Output as JSON
  traverse status req_12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		requestID := args[0]

		if statusWatch {
			return watchStatus(requestID)
		}

		return showStatus(requestID)
	},
}

func showStatus(requestID string) error {
	status, err := traverseClient.GetStatus(requestID)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if statusJSON {
		return formatJSON(status)
	}

	// Print formatted status
	fmt.Printf("Request ID: %s\n", status.RequestID)
	fmt.Printf("Status: %s\n", formatStatus(status.Status))
	if status.StatusDetail != "" {
		fmt.Printf("Detail: %s\n", status.StatusDetail)
	}
	fmt.Printf("\nRequest Details:\n")
	fmt.Printf("  Secret Path: %s\n", status.SecretPath)
	fmt.Printf("  Client ID: %s\n", status.ClientID)
	fmt.Printf("  Reason: %s\n", status.Reason)
	fmt.Printf("  Created: %s\n", status.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Expires: %s\n", status.ExpiresAt.Format(time.RFC3339))

	if status.Status == "approved" {
		fmt.Printf("\nApproval Details:\n")
		if status.ApprovedAt != nil {
			fmt.Printf("  Approved At: %s\n", status.ApprovedAt.Format(time.RFC3339))
		}
		if len(status.ApprovedBy) > 0 {
			fmt.Printf("  Approved By: %s\n", status.ApprovedBy[0].Identity)
		}
		if status.Token != "" {
			fmt.Printf("\nAccess Token:\n")
			fmt.Printf("  Token: %s...%s\n", status.Token[:8], status.Token[len(status.Token)-8:])
			if status.TokenExpiresAt != nil {
				fmt.Printf("  Expires: %s\n", status.TokenExpiresAt.Format(time.RFC3339))
			}
			if status.SecretURL != "" {
				fmt.Printf("\nRetrieve secret with:\n")
				fmt.Printf("  traverse get %s --token %s\n", status.SecretPath, status.Token)
			}
		}
	}

	if status.Status == "denied" {
		fmt.Printf("\nDenial Details:\n")
		if status.DeniedAt != nil {
			fmt.Printf("  Denied At: %s\n", status.DeniedAt.Format(time.RFC3339))
		}
		if len(status.DeniedBy) > 0 {
			fmt.Printf("  Denied By: %s\n", status.DeniedBy[0].Identity)
			if status.DeniedBy[0].Reason != "" {
				fmt.Printf("  Reason: %s\n", status.DeniedBy[0].Reason)
			}
		}
	}

	return nil
}

func watchStatus(requestID string) error {
	fmt.Printf("Watching request %s (press Ctrl+C to stop)...\n\n", requestID)

	lastStatus := ""
	for {
		status, err := traverseClient.GetStatus(requestID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting status: %v\n", err)
			time.Sleep(statusInterval)
			continue
		}

		// Only print if status changed
		if status.Status != lastStatus {
			timestamp := time.Now().Format("15:04:05")
			fmt.Printf("[%s] Status: %s\n", timestamp, formatStatus(status.Status))

			if status.Status == "approved" {
				fmt.Printf("\nRequest approved!\n")
				if status.Token != "" {
					fmt.Printf("Token is available. Use 'traverse get' to retrieve the secret.\n")
				}
				return nil
			}

			if status.Status == "denied" {
				fmt.Printf("\nRequest denied.\n")
				return fmt.Errorf("request was denied")
			}

			if status.Status == "expired" {
				fmt.Printf("\nRequest expired.\n")
				return fmt.Errorf("request expired")
			}

			lastStatus = status.Status
		}

		time.Sleep(statusInterval)
	}
}

// formatStatus returns a colored/formatted status string
func formatStatus(status string) string {
	switch status {
	case "approved":
		return "✓ approved"
	case "denied":
		return "✗ denied"
	case "pending":
		return "⏳ pending"
	case "expired":
		return "⌛ expired"
	default:
		return status
	}
}

func init() {
	RootCmd.AddCommand(statusCmd)

	// Flags
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "Watch for status changes")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")
	statusCmd.Flags().DurationVarP(&statusInterval, "interval", "i", 5*time.Second, "Poll interval when watching")
}
