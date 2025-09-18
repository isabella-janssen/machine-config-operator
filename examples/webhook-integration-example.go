// This file shows how to integrate the MCP deletion webhook into the machine config controller
// This would be added to cmd/machine-config-controller/start.go

package main

import (
	// ... existing imports ...
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
)

// Add this to the run function in cmd/machine-config-controller/start.go
// around line 148, after the other controllers are initialized:

func integrateWebhook(ctrlctx *ctrlcommon.ControllerContext, runContext context.Context) {
	// Initialize the webhook manager
	webhookManager, err := ctrlcommon.NewWebhookManager(
		ctrlctx.ClientBuilder.KubeClientOrDie("mcp-deletion-webhook"),
		ctrlctx.ClientBuilder.MachineConfigClientOrDie("mcp-deletion-webhook"),
		ctrlctx.ClientBuilder.MachineClientOrDie("mcp-deletion-webhook"),
	)
	if err != nil {
		klog.Fatalf("Failed to create webhook manager: %v", err)
	}

	// Start the webhook server
	go func() {
		if err := webhookManager.Start(runContext); err != nil {
			klog.Errorf("Webhook manager failed: %v", err)
		}
	}()

	// Ensure webhook manager is stopped when context is cancelled
	go func() {
		<-runContext.Done()
		webhookManager.Stop()
	}()
}

// Example of how to modify the existing run function:
func runWithWebhook(ctx context.Context) {
	// ... existing controller initialization code ...

	// Add this after line 148 in the original start.go:
	integrateWebhook(ctrlctx, ctx)

	// ... rest of the existing run function ...
}
