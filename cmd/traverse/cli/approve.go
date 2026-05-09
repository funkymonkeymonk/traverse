package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/funkymonkeymonk/traverse/pkg/client"
	"github.com/spf13/cobra"
)

var (
	approveReason string
	approveJSON   bool
	approveList   bool
	approveAll    bool
)

// approveCmd represents the approve command
var approveCmd = &cobra.Command{
	Use:   "approve [request-id]",
	Short: "Approve a pending secret request",
	Long: `Approve a pending secret request. You can provide an optional reason
for the approval. If no request ID is provided with --list, shows all pending requests.`,
	Example: `  # Approve a specific request
  traverse approve req_12345

  # Approve with a reason
  traverse approve req_12345 --reason "Valid business need"

  # List all pending requests
  traverse approve --list

  # Approve all pending requests
  traverse approve --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if approveList {
			return listPendingRequests()
		}

		if approveAll {
			return approveAllRequests()
		}

		if len(args) == 0 {
			return fmt.Errorf("request ID required (or use --list to see pending requests)")
		}

		requestID := args[0]
		return approveRequest(requestID)
	},
}

func approveRequest(requestID string) error {
	// Get current status first
	status, err := traverseClient.GetStatus(requestID)
	if err != nil {
		return fmt.Errorf("failed to get request status: %w", err)
	}

	if status.Status != "pending" {
		return fmt.Errorf("request %s is already %s", requestID, status.Status)
	}

	req := client.ApproveRequestRequest{
		Reason: approveReason,
	}

	resp, err := traverseClient.ApproveRequest(requestID, req)
	if err != nil {
		return fmt.Errorf("failed to approve request: %w", err)
	}

	if approveJSON {
		return formatJSON(resp)
	}

	fmt.Printf("✓ Request approved\n")
	fmt.Printf("Request ID: %s\n", resp.RequestID)
	fmt.Printf("Approved at: %s\n", resp.ApprovedAt.Format(time.RFC3339))

	if resp.RemainingRequiredApprovals > 0 {
		fmt.Printf("\nNote: %d more approval(s) required\n", resp.RemainingRequiredApprovals)
	} else {
		fmt.Printf("\nRequest is now fully approved and token has been issued.\n")
	}

	return nil
}

func listPendingRequests() error {
	// For now, this is a placeholder - in a real implementation,
	// we'd need an API endpoint to list pending requests
	fmt.Println("Pending requests:")
	fmt.Println()
	fmt.Println("(Note: This would show pending requests from the API)")
	fmt.Println()
	fmt.Println("Use 'traverse approve <request-id>' to approve a specific request")
	return nil
}

func approveAllRequests() error {
	// For now, this is a placeholder
	return fmt.Errorf("approve all not yet implemented")
}

// denyCmd represents the deny command
var denyCmd = &cobra.Command{
	Use:   "deny [request-id]",
	Short: "Deny a pending secret request",
	Long: `Deny a pending secret request. A reason must be provided for the denial.`,
	Example: `  # Deny a request with reason
  traverse deny req_12345 --reason "Insufficient justification"

  # Deny without reason flag (will prompt)
  traverse deny req_12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		requestID := args[0]
		return denyRequest(requestID)
	},
}

var (
	denyReason string
	denyJSON   bool
)

func denyRequest(requestID string) error {
	// Get current status first
	status, err := traverseClient.GetStatus(requestID)
	if err != nil {
		return fmt.Errorf("failed to get request status: %w", err)
	}

	if status.Status != "pending" {
		return fmt.Errorf("request %s is already %s", requestID, status.Status)
	}

	// Check if reason is provided
	if denyReason == "" {
		// Prompt for reason
		fmt.Print("Reason for denial (required): ")
		var input string
		fmt.Scanln(&input)
		denyReason = strings.TrimSpace(input)

		if denyReason == "" {
			return fmt.Errorf("reason is required for denial")
		}
	}

	req := client.DenyRequestRequest{
		Reason: denyReason,
	}

	resp, err := traverseClient.DenyRequest(requestID, req)
	if err != nil {
		return fmt.Errorf("failed to deny request: %w", err)
	}

	if denyJSON {
		return formatJSON(resp)
	}

	fmt.Printf("✓ Request denied\n")
	fmt.Printf("Request ID: %s\n", resp.RequestID)
	fmt.Printf("Denied at: %s\n", resp.DeniedAt.Format(time.RFC3339))

	return nil
}

func init() {
	RootCmd.AddCommand(approveCmd)
	RootCmd.AddCommand(denyCmd)

	// Approve flags
	approveCmd.Flags().StringVarP(&approveReason, "reason", "r", "", "Reason for approval")
	approveCmd.Flags().BoolVar(&approveJSON, "json", false, "Output as JSON")
	approveCmd.Flags().BoolVarP(&approveList, "list", "l", false, "List pending requests")
	approveCmd.Flags().BoolVarP(&approveAll, "all", "a", false, "Approve all pending requests")

	// Deny flags
	denyCmd.Flags().StringVarP(&denyReason, "reason", "r", "", "Reason for denial (required)")
	denyCmd.MarkFlagRequired("reason")
	denyCmd.Flags().BoolVar(&denyJSON, "json", false, "Output as JSON")
}
