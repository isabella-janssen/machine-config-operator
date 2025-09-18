// This file shows how to integrate the MCP deletion controller into the machine config controller
// This would be added to cmd/machine-config-controller/start.go

package main

import (
	// ... existing imports ...
	mcpdeletionctrl "github.com/openshift/machine-config-operator/pkg/controller/mcp-deletion"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
)

// Add this to the run function in cmd/machine-config-controller/start.go
// around line 190, after the other controllers are started:

func integrateMCPDeletionController(ctrlctx *ctrlcommon.ControllerContext, stopCh <-chan struct{}) {
	// Create the MCP deletion controller
	mcpDeletionController := mcpdeletionctrl.NewController(
		ctrlctx.ClientBuilder.KubeClientOrDie("mcp-deletion-controller"),
		ctrlctx.ClientBuilder.MachineConfigClientOrDie("mcp-deletion-controller"),
		ctrlctx.ClientBuilder.MachineClientOrDie("mcp-deletion-controller"), // May be nil if not available
		ctrlctx.InformerFactory.Machineconfiguration().V1().MachineConfigPools(),
		ctrlctx.MachineInformerFactory.Machine().V1beta1().Machines(), // May be nil if not available
	)
	
	// Start the controller
	go mcpDeletionController.Run(1, stopCh) // Single worker should be sufficient
	
	klog.Info("Started MCP deletion protection controller")
}

// Alternative integration if machine client/informers are not available:

func integrateMCPDeletionControllerBasic(ctrlctx *ctrlcommon.ControllerContext, stopCh <-chan struct{}) {
	// Create the MCP deletion controller without machine informers
	mcpDeletionController := mcpdeletionctrl.NewController(
		ctrlctx.InformerFactory.Machineconfiguration().V1().MachineConfigPools(),
		nil, // No machine informer
		ctrlctx.ClientBuilder.KubeClientOrDie("mcp-deletion-controller"),
		ctrlctx.ClientBuilder.MachineConfigClientOrDie("mcp-deletion-controller"),
		nil, // No machine client - controller will handle this gracefully
	)
	
	// Start the controller
	go mcpDeletionController.Run(1, stopCh)
	
	klog.Info("Started MCP deletion protection controller (basic mode - machine checking disabled)")
}

// Add this call in the run function after other controllers are started:
// integrateMCPDeletionController(ctrlctx, stopCh)

/* Example placement in start.go:

func run(ctx context.Context, controllerConfig *mcfgv1.ControllerConfig, runContext ControllerContext) error {
	// ... existing controller initialization ...
	
	// Start template controller
	go template.NewController(
		runContext.templateInformer,
		runContext.clientBuilder.KubeClientOrDie("template-controller"),
		runContext.clientBuilder.MachineConfigClientOrDie("template-controller"),
		runContext.eventRecorder,
	).Run(1, stopCh)
	
	// ... other controllers ...
	
	// ADD THIS: Start MCP deletion protection controller
	integrateMCPDeletionController(&runContext, stopCh)
	
	// ... rest of function ...
}
*/
