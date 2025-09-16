package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcfgclientset "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	machineclientset "github.com/openshift/client-go/machine/clientset/versioned"
	daemonconsts "github.com/openshift/machine-config-operator/pkg/daemon/constants"
)

// MCPDeletionWebhook validates MachineConfigPool deletion requests
type MCPDeletionWebhook struct {
	kubeClient    kubernetes.Interface
	mcfgClient    mcfgclientset.Interface
	machineClient machineclientset.Interface
}

// NewMCPDeletionWebhook creates a new MCP deletion validation webhook
func NewMCPDeletionWebhook(kubeClient kubernetes.Interface, mcfgClient mcfgclientset.Interface, machineClient machineclientset.Interface) *MCPDeletionWebhook {
	return &MCPDeletionWebhook{
		kubeClient:    kubeClient,
		mcfgClient:    mcfgClient,
		machineClient: machineClient,
	}
}

// ValidateMCPDeletion handles the admission webhook validation
func (w *MCPDeletionWebhook) ValidateMCPDeletion(ctx context.Context, req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	klog.V(4).Infof("Validating MachineConfigPool deletion: %s", req.Name)

	// Only validate DELETE operations
	if req.Operation != admissionv1.Delete {
		return &admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: true,
		}
	}

	// Get the MCP being deleted from oldObject
	var mcp mcfgv1.MachineConfigPool
	if err := json.Unmarshal(req.OldObject.Raw, &mcp); err != nil {
		klog.Errorf("Failed to unmarshal MachineConfigPool: %v", err)
		return &admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: false,
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Failed to unmarshal MachineConfigPool: %v", err),
			},
		}
	}

	// Skip validation for default pools (master, worker)
	if isDefaultPool(mcp.Name) {
		klog.V(4).Infof("Skipping validation for default pool: %s", mcp.Name)
		return &admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: true,
		}
	}

	// Perform the validation checks
	if err := w.validateMCPDeletion(ctx, &mcp); err != nil {
		klog.Infof("Blocking deletion of MachineConfigPool %s: %v", mcp.Name, err)
		return &admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: false,
			Result: &metav1.Status{
				Code:    http.StatusForbidden,
				Message: fmt.Sprintf("Cannot delete MachineConfigPool '%s': %v", mcp.Name, err),
			},
		}
	}

	klog.V(4).Infof("Allowing deletion of MachineConfigPool: %s", mcp.Name)
	return &admissionv1.AdmissionResponse{
		UID:     req.UID,
		Allowed: true,
	}
}

// validateMCPDeletion performs the actual validation logic
func (w *MCPDeletionWebhook) validateMCPDeletion(ctx context.Context, mcp *mcfgv1.MachineConfigPool) error {
	// Condition 1: Check if there are still machines assigned to this MCP
	if err := w.checkMachinesInPool(ctx, mcp); err != nil {
		return fmt.Errorf("machines still exist in pool: %w", err)
	}

	// Condition 2: Check if there are eligible nodes still targeting this MCP's rendered config
	if err := w.checkNodesTargetingMCPConfig(ctx, mcp); err != nil {
		return fmt.Errorf("nodes still targeting pool configuration: %w", err)
	}

	return nil
}

// checkMachinesInPool verifies no machines are still assigned to the MCP
func (w *MCPDeletionWebhook) checkMachinesInPool(ctx context.Context, mcp *mcfgv1.MachineConfigPool) error {
	// Get all machines in the openshift-machine-api namespace
	machines, err := w.machineClient.MachineV1beta1().Machines("openshift-machine-api").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list machines: %w", err)
	}

	// Check each machine to see if it targets this MCP
	for _, machine := range machines.Items {
		if machineTargetsMCP(&machine, mcp.Name) {
			return fmt.Errorf("machine '%s' is still assigned to this pool", machine.Name)
		}
	}

	return nil
}

// checkNodesTargetingMCPConfig verifies no eligible nodes are still targeting the MCP's rendered config
func (w *MCPDeletionWebhook) checkNodesTargetingMCPConfig(ctx context.Context, mcp *mcfgv1.MachineConfigPool) error {
	// Get all nodes
	nodes, err := w.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Check each node
	for _, node := range nodes.Items {
		// Skip non-eligible nodes (master, arbiter)
		if !isEligibleNode(&node) {
			continue
		}

		if nodeTargetsMCPConfig(&node, mcp.Name) {
			return fmt.Errorf("node '%s' is still targeting rendered configuration from this pool", node.Name)
		}
	}

	return nil
}

// isDefaultPool checks if the pool is a default system pool that shouldn't be validated
func isDefaultPool(poolName string) bool {
	defaultPools := []string{"master", "worker"}
	for _, defaultPool := range defaultPools {
		if poolName == defaultPool {
			return true
		}
	}
	return false
}

// machineTargetsMCP checks if a machine is configured to create nodes for the specified MCP
func machineTargetsMCP(machine *machinev1beta1.Machine, mcpName string) bool {
	// Check if the machine has the role label that would make nodes join this MCP
	if roleLabel, exists := machine.Labels["machine.openshift.io/cluster-api-machine-role"]; exists {
		if roleLabel == mcpName {
			return true
		}
	}

	// Also check machine role labels that might indicate MCP targeting
	roleKey := fmt.Sprintf("machine.openshift.io/cluster-api-machine-type")
	if roleType, exists := machine.Labels[roleKey]; exists {
		if roleType == mcpName {
			return true
		}
	}

	return false
}

// isEligibleNode checks if a node is eligible to be part of a custom MCP
// (not master, not arbiter)
func isEligibleNode(node *corev1.Node) bool {
	// Skip master nodes
	if _, isMaster := node.Labels["node-role.kubernetes.io/master"]; isMaster {
		return false
	}

	// Skip arbiter nodes
	if _, isArbiter := node.Labels["node-role.kubernetes.io/arbiter"]; isArbiter {
		return false
	}

	return true
}

// nodeTargetsMCPConfig checks if a node is currently targeting or has targeted the MCP's rendered config
func nodeTargetsMCPConfig(node *corev1.Node, mcpName string) bool {
	renderedConfigPrefix := fmt.Sprintf("rendered-%s-", mcpName)

	// Check current config annotation
	if currentConfig, exists := node.Annotations[daemonconsts.CurrentMachineConfigAnnotationKey]; exists {
		if strings.HasPrefix(currentConfig, renderedConfigPrefix) {
			return true
		}
	}

	// Check desired config annotation
	if desiredConfig, exists := node.Annotations[daemonconsts.DesiredMachineConfigAnnotationKey]; exists {
		if strings.HasPrefix(desiredConfig, renderedConfigPrefix) {
			return true
		}
	}

	return false
}

// ServeHTTP implements the HTTP handler for the webhook
func (w *MCPDeletionWebhook) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var body []byte
	if request.Body != nil {
		if data, err := io.ReadAll(request.Body); err == nil {
			body = data
		}
	}

	// Verify content type
	contentType := request.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Expected content type application/json, got %s", contentType)
		http.Error(writer, "Expected content type application/json", http.StatusBadRequest)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	var admissionReview admissionv1.AdmissionReview

	if err := json.Unmarshal(body, &admissionReview); err != nil {
		klog.Errorf("Failed to unmarshal admission review: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Failed to unmarshal admission review: %v", err),
			},
		}
	} else {
		admissionResponse = w.ValidateMCPDeletion(request.Context(), admissionReview.Request)
	}

	admissionReview.Response = admissionResponse
	if admissionReview.Request != nil {
		admissionReview.Response.UID = admissionReview.Request.UID
	}

	respBytes, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Errorf("Failed to marshal admission response: %v", err)
		http.Error(writer, fmt.Sprintf("Failed to marshal admission response: %v", err), http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	if _, err := writer.Write(respBytes); err != nil {
		klog.Errorf("Failed to write response: %v", err)
	}
}
