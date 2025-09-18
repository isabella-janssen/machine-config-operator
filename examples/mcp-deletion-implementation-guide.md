# MCP Deletion Controller Implementation Guide

This guide shows how to implement the validation rules for the MCP deletion finalizer-based controller.

## Overview

The MCP deletion controller uses finalizers to prevent unsafe deletion of custom MachineConfigPools. When a custom MCP is created, a finalizer is added. When deletion is requested, the controller validates safety conditions before removing the finalizer and allowing deletion.

## Architecture

```
MCP Creation → Add Finalizer → MCP exists with protection
      ↓
MCP Deletion Request → DeletionTimestamp set → Controller validates
      ↓                                              ↓
  Validation Fails                            Validation Passes  
      ↓                                              ↓
Finalizer remains → Deletion blocked          Remove Finalizer → Deletion proceeds
```

## Implementation Tasks

### 1. Machine Checking Logic (`checkForActiveMachines`)

**File**: `pkg/controller/mcp-deletion/mcp_deletion_controller.go:L206`

**Requirements**: Check if any machines are still configured to create nodes for this MCP.

**Implementation Steps**:

```go
func (ctrl *Controller) checkForActiveMachines(pool *mcfgv1.MachineConfigPool) (bool, string, error) {
    // Skip if machine client not available
    if ctrl.machineClient == nil {
        klog.Warningf("Machine client not available, skipping machine check for MCP %s", pool.Name)
        return false, "", nil
    }

    // List all machines in openshift-machine-api namespace
    machines, err := ctrl.machineClient.MachineV1beta1().Machines("openshift-machine-api").List(
        context.TODO(), 
        metav1.ListOptions{},
    )
    if err != nil {
        // Handle error - maybe machines namespace doesn't exist yet
        if errors.IsNotFound(err) {
            klog.V(4).Infof("openshift-machine-api namespace not found, no machines to check")
            return false, "", nil
        }
        return false, "", fmt.Errorf("failed to list machines: %w", err)
    }

    var blockingMachines []string

    for _, machine := range machines.Items {
        // Check method 1: Direct machine role label
        if machine.Labels != nil {
            if role, exists := machine.Labels["machine.openshift.io/cluster-api-machine-role"]; exists && role == pool.Name {
                blockingMachines = append(blockingMachines, machine.Name)
                continue
            }
        }

        // Check method 2: Machine would create nodes matching this pool's nodeSelector
        if wouldMatchPool, err := ctrl.machineWouldMatchPool(&machine, pool); err != nil {
            klog.Warningf("Failed to check if machine %s would match pool %s: %v", machine.Name, pool.Name, err)
            continue
        } else if wouldMatchPool {
            blockingMachines = append(blockingMachines, machine.Name)
        }
    }

    if len(blockingMachines) > 0 {
        return true, fmt.Sprintf("Machines still configured for this pool: %v", blockingMachines), nil
    }

    return false, "", nil
}

// Helper function to check if a machine would create nodes matching the pool
func (ctrl *Controller) machineWouldMatchPool(machine *machinev1beta1.Machine, pool *mcfgv1.MachineConfigPool) (bool, error) {
    // TODO: Parse machine's providerSpec to determine what node labels it would create
    // This is platform-specific and complex. For MVP, you might skip this and rely on
    // the machine role label check above.

    // For now, return false (skip this advanced check)
    return false, nil
}
```

### 2. Node Targeting Check Logic (`checkForTargetingNodes`)

**File**: `pkg/controller/mcp-deletion/mcp_deletion_controller.go:L251`

**Requirements**: Check if any eligible nodes are still using this MCP's rendered configuration.

**Implementation Steps**:

```go
func (ctrl *Controller) checkForTargetingNodes(pool *mcfgv1.MachineConfigPool) (bool, string, error) {
    // List all nodes in the cluster
    nodes, err := ctrl.kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        return false, "", fmt.Errorf("failed to list nodes: %w", err)
    }

    // Get the expected rendered config prefix for this pool
    renderedConfigPrefix := "rendered-" + pool.Name + "-"
    var blockingNodes []string

    for _, node := range nodes.Items {
        // Skip non-eligible nodes
        if !ctrl.isNodeEligibleForCustomMCP(&node) {
            continue
        }

        // Check if node is currently using this pool's config
        if ctrl.nodeTargetsPool(&node, renderedConfigPrefix) {
            reason := ctrl.getNodeTargetingReason(&node, renderedConfigPrefix)
            blockingNodes = append(blockingNodes, fmt.Sprintf("%s (%s)", node.Name, reason))
        }
    }

    if len(blockingNodes) > 0 {
        return true, fmt.Sprintf("Eligible nodes still targeting this pool's config: %v", blockingNodes), nil
    }

    return false, "", nil
}

// Helper function to check if node is eligible for custom MCPs
func (ctrl *Controller) isNodeEligibleForCustomMCP(node *corev1.Node) bool {
    // Skip master nodes
    if _, isMaster := node.Labels["node-role.kubernetes.io/master"]; isMaster {
        return false
    }

    // Skip arbiter nodes  
    if _, isArbiter := node.Labels["node-role.kubernetes.io/arbiter"]; isArbiter {
        return false
    }

    // Add other exclusions as needed (e.g., special system nodes)

    return true
}

// Helper function to check if node targets the pool's config
func (ctrl *Controller) nodeTargetsPool(node *corev1.Node, renderedConfigPrefix string) bool {
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

// Helper function to get detailed reason why node is blocking deletion
func (ctrl *Controller) getNodeTargetingReason(node *corev1.Node, renderedConfigPrefix string) string {
    var reasons []string

    if currentConfig, exists := node.Annotations[daemonconsts.CurrentMachineConfigAnnotationKey]; exists {
        if strings.HasPrefix(currentConfig, renderedConfigPrefix) {
            reasons = append(reasons, fmt.Sprintf("current: %s", currentConfig))
        }
    }

    if desiredConfig, exists := node.Annotations[daemonconsts.DesiredMachineConfigAnnotationKey]; exists {
        if strings.HasPrefix(desiredConfig, renderedConfigPrefix) {
            reasons = append(reasons, fmt.Sprintf("desired: %s", desiredConfig))
        }
    }

    return strings.Join(reasons, ", ")
}
```

## Usage Examples

### Testing the Controller

```bash
# 1. Create a custom MCP
cat <<EOF | kubectl apply -f -
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfigPool
metadata:
  name: custom-role
spec:
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/custom-role: ""
  machineConfigSelector:
    matchLabels:
      machineconfiguration.openshift.io/role: custom-role
EOF

# 2. Verify finalizer was added
kubectl get mcp custom-role -o jsonpath='{.metadata.finalizers}'

# 3. Try to delete (should be blocked if machines/nodes are present)
kubectl delete mcp custom-role

# 4. Check events for blocking reason
kubectl get events --field-selector involvedObject.name=custom-role

# 5. Clean up machines/nodes, then delete should succeed
```

### Force Deletion (Bypass)

If you need to force delete an MCP (bypassing validation):

```bash
# Remove the finalizer manually
kubectl patch mcp custom-role --type='merge' -p='{"metadata":{"finalizers":null}}'

# Or edit directly
kubectl edit mcp custom-role
# Remove the finalizer line and save
```

### Monitoring

```bash
# Watch MCP deletion events
kubectl get events -w --field-selector involvedObject.kind=MachineConfigPool

# Check controller logs
kubectl logs -n openshift-machine-config-operator deployment/machine-config-controller | grep "MCPDeletionController"
```

## Error Scenarios

### Machine Client Unavailable
- Controller gracefully handles missing machine client
- Only validates nodes in this case
- Logs warning about limited validation

### Node List Failure
- Returns error, prevents deletion
- Fail-safe approach - better to be conservative

### Partial Validation Failure
- Individual machine/node check failures are logged but don't block the overall decision
- Only total failures prevent deletion

## Integration Notes

1. **Controller Startup**: Add to machine-config-controller start sequence
2. **RBAC**: Ensure controller has permissions to list/patch MCPs, list nodes/machines
3. **Monitoring**: Add metrics for blocked deletions, validation errors
4. **Logging**: Use structured logging for better observability

## Future Enhancements

1. **Webhook Validation**: Add complementary admission webhook for immediate feedback
2. **Machine Provider Parsing**: Implement platform-specific machine providerSpec parsing
3. **Grace Period**: Add configurable grace period before final validation
4. **Metrics**: Add Prometheus metrics for monitoring deletion attempts
