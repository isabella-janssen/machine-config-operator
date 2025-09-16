package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	fakemachineclient "github.com/openshift/client-go/machine/clientset/versioned/fake"
	fakemcfgclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned/fake"
	daemonconsts "github.com/openshift/machine-config-operator/pkg/daemon/constants"
)

func TestMCPDeletionWebhook_ValidateMCPDeletion_DefaultPools(t *testing.T) {
	webhook := createTestWebhook(nil, nil, nil)
	
	testCases := []struct {
		name     string
		poolName string
		expected bool
	}{
		{
			name:     "Allow deletion of master pool",
			poolName: "master",
			expected: true,
		},
		{
			name:     "Allow deletion of worker pool",
			poolName: "worker",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mcp := createTestMCP(tc.poolName)
			req := createAdmissionRequest(mcp, admissionv1.Delete)
			
			response := webhook.ValidateMCPDeletion(context.Background(), req)
			
			if response.Allowed != tc.expected {
				t.Errorf("Expected allowed=%v, got %v", tc.expected, response.Allowed)
			}
		})
	}
}

func TestMCPDeletionWebhook_ValidateMCPDeletion_CustomPoolWithMachines(t *testing.T) {
	// Create a machine that targets the custom pool
	machine := &machinev1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine",
			Namespace: "openshift-machine-api",
			Labels: map[string]string{
				"machine.openshift.io/cluster-api-machine-role": "custom",
			},
		},
	}
	
	machines := []runtime.Object{machine}
	webhook := createTestWebhook(machines, nil, nil)
	
	mcp := createTestMCP("custom")
	req := createAdmissionRequest(mcp, admissionv1.Delete)
	
	response := webhook.ValidateMCPDeletion(context.Background(), req)
	
	if response.Allowed {
		t.Error("Expected deletion to be blocked due to existing machines")
	}
	
	if response.Result == nil || response.Result.Message == "" {
		t.Error("Expected error message explaining why deletion was blocked")
	}
}

func TestMCPDeletionWebhook_ValidateMCPDeletion_CustomPoolWithNodesTargetingConfig(t *testing.T) {
	// Create a node targeting the custom pool's rendered config
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				daemonconsts.CurrentMachineConfigAnnotationKey: "rendered-custom-abc123",
			},
			Labels: map[string]string{
				// Not a master or arbiter node
				"node-role.kubernetes.io/worker": "",
			},
		},
	}
	
	nodes := []runtime.Object{node}
	webhook := createTestWebhook(nil, nodes, nil)
	
	mcp := createTestMCP("custom")
	req := createAdmissionRequest(mcp, admissionv1.Delete)
	
	response := webhook.ValidateMCPDeletion(context.Background(), req)
	
	if response.Allowed {
		t.Error("Expected deletion to be blocked due to nodes targeting rendered config")
	}
	
	if response.Result == nil || response.Result.Message == "" {
		t.Error("Expected error message explaining why deletion was blocked")
	}
}

func TestMCPDeletionWebhook_ValidateMCPDeletion_CustomPoolWithMasterNodes(t *testing.T) {
	// Create a master node targeting the custom pool's rendered config
	// This should NOT block deletion since master nodes are not eligible
	masterNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "master-node",
			Annotations: map[string]string{
				daemonconsts.CurrentMachineConfigAnnotationKey: "rendered-custom-abc123",
			},
			Labels: map[string]string{
				"node-role.kubernetes.io/master": "",
			},
		},
	}
	
	nodes := []runtime.Object{masterNode}
	webhook := createTestWebhook(nil, nodes, nil)
	
	mcp := createTestMCP("custom")
	req := createAdmissionRequest(mcp, admissionv1.Delete)
	
	response := webhook.ValidateMCPDeletion(context.Background(), req)
	
	if !response.Allowed {
		t.Error("Expected deletion to be allowed since master nodes are not eligible")
	}
}

func TestMCPDeletionWebhook_ValidateMCPDeletion_CustomPoolClean(t *testing.T) {
	// Test successful deletion when no machines or eligible nodes are targeting the pool
	
	// Create a node NOT targeting the custom pool
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				daemonconsts.CurrentMachineConfigAnnotationKey: "rendered-worker-xyz789",
			},
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
		},
	}
	
	// Create a machine NOT targeting the custom pool
	machine := &machinev1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine",
			Namespace: "openshift-machine-api",
			Labels: map[string]string{
				"machine.openshift.io/cluster-api-machine-role": "worker",
			},
		},
	}
	
	machines := []runtime.Object{machine}
	nodes := []runtime.Object{node}
	webhook := createTestWebhook(machines, nodes, nil)
	
	mcp := createTestMCP("custom")
	req := createAdmissionRequest(mcp, admissionv1.Delete)
	
	response := webhook.ValidateMCPDeletion(context.Background(), req)
	
	if !response.Allowed {
		t.Errorf("Expected deletion to be allowed for clean custom pool, got error: %v", 
			response.Result.Message)
	}
}

func TestMCPDeletionWebhook_ValidateMCPDeletion_NonDeleteOperation(t *testing.T) {
	webhook := createTestWebhook(nil, nil, nil)
	
	mcp := createTestMCP("custom")
	req := createAdmissionRequest(mcp, admissionv1.Update)
	
	response := webhook.ValidateMCPDeletion(context.Background(), req)
	
	if !response.Allowed {
		t.Error("Expected non-delete operations to always be allowed")
	}
}

func TestNodeTargetsMCPConfig(t *testing.T) {
	testCases := []struct {
		name        string
		annotations map[string]string
		mcpName     string
		expected    bool
	}{
		{
			name: "Current config targets MCP",
			annotations: map[string]string{
				daemonconsts.CurrentMachineConfigAnnotationKey: "rendered-custom-abc123",
			},
			mcpName:  "custom",
			expected: true,
		},
		{
			name: "Desired config targets MCP",
			annotations: map[string]string{
				daemonconsts.DesiredMachineConfigAnnotationKey: "rendered-custom-def456",
			},
			mcpName:  "custom",
			expected: true,
		},
		{
			name: "Config does not target MCP",
			annotations: map[string]string{
				daemonconsts.CurrentMachineConfigAnnotationKey: "rendered-worker-xyz789",
			},
			mcpName:  "custom",
			expected: false,
		},
		{
			name:        "No config annotations",
			annotations: map[string]string{},
			mcpName:     "custom",
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}
			
			result := nodeTargetsMCPConfig(node, tc.mcpName)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestIsEligibleNode(t *testing.T) {
	testCases := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name: "Worker node is eligible",
			labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			expected: true,
		},
		{
			name: "Master node is not eligible",
			labels: map[string]string{
				"node-role.kubernetes.io/master": "",
			},
			expected: false,
		},
		{
			name: "Arbiter node is not eligible",
			labels: map[string]string{
				"node-role.kubernetes.io/arbiter": "",
			},
			expected: false,
		},
		{
			name:     "Node with no role labels is eligible",
			labels:   map[string]string{},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tc.labels,
				},
			}
			
			result := isEligibleNode(node)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// Helper functions

func createTestWebhook(machines, nodes, mcps []runtime.Object) *MCPDeletionWebhook {
	kubeClient := fake.NewSimpleClientset(nodes...)
	mcfgClient := fakemcfgclient.NewSimpleClientset(mcps...)
	machineClient := fakemachineclient.NewSimpleClientset(machines...)
	
	return NewMCPDeletionWebhook(kubeClient, mcfgClient, machineClient)
}

func createTestMCP(name string) *mcfgv1.MachineConfigPool {
	return &mcfgv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: mcfgv1.MachineConfigPoolSpec{
			Configuration: mcfgv1.MachineConfigPoolSpecConfiguration{
				Name: fmt.Sprintf("rendered-%s-abc123", name),
			},
		},
	}
}

func createAdmissionRequest(mcp *mcfgv1.MachineConfigPool, operation admissionv1.Operation) *admissionv1.AdmissionRequest {
	mcpBytes, _ := json.Marshal(mcp)
	
	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Kind:      metav1.GroupVersionKind{Group: "machineconfiguration.openshift.io", Version: "v1", Kind: "MachineConfigPool"},
		Resource:  metav1.GroupVersionResource{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigpools"},
		Name:      mcp.Name,
		Namespace: "",
		Operation: operation,
	}
	
	if operation == admissionv1.Delete {
		req.OldObject = runtime.RawExtension{Raw: mcpBytes}
	} else {
		req.Object = runtime.RawExtension{Raw: mcpBytes}
	}
	
	return req
}
