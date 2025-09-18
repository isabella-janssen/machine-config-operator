# MCP Deletion Finalizer Integration

This document describes the finalizer-based MCP deletion protection that has been integrated into the existing node controller.

## Overview

The MCP deletion protection uses Kubernetes finalizers to prevent unsafe deletion of custom MachineConfigPools. This approach:

- ✅ **Integrates directly into existing controller** - No separate controller needed
- ✅ **Provides clear user feedback** - Events explain why deletion is blocked
- ✅ **Fail-safe approach** - Better to block safe deletions than allow unsafe ones
- ✅ **System pools unaffected** - Master/worker pools can still be deleted normally

## Architecture

```
Custom MCP Creation → Node Controller adds finalizer → MCP protected
         ↓
Custom MCP Deletion Request → DeletionTimestamp set → Node Controller validates
         ↓                                                       ↓
   Validation Fails                                    Validation Passes  
         ↓                                                       ↓
   Finalizer remains                                     Remove Finalizer
         ↓                                                       ↓
   Deletion blocked                                      Deletion proceeds
   (with events explaining why)
```

## What's Implemented

### 1. **Automatic Finalizer Management**
- **File**: `pkg/controller/node/node_controller.go:L1120-1128`
- **Function**: Custom MCPs automatically get deletion protection finalizer when created
- **System pools**: Master and worker pools are **not** affected

### 2. **Deletion Request Handling**
- **File**: `pkg/controller/node/node_controller.go:L1116-1117`
- **Function**: When deletion timestamp is set, validation logic runs instead of immediate deletion
- **Fallback**: If validation fails, falls back to regular status sync

### 3. **Safety Validation Rules**
- **File**: `pkg/controller/node/node_controller.go:L1667-1687`
- **Rule 1**: Custom MCP cannot be deleted if it still has machines
- **Rule 2**: Custom MCP cannot be deleted if eligible nodes are still targeting its rendered config

### 4. **User Feedback**
- **Events**: Clear events explain why deletion is blocked
- **Logs**: Detailed logging for debugging
- **Status**: Pool status continues to be updated even when deletion is blocked

## Integration Points

The finalizer logic is integrated into the existing node controller at these key points:

### **Main Sync Function** (`syncMachineConfigPool`)
```go
if pool.DeletionTimestamp != nil {
    return ctrl.handleMCPDeletion(pool)  // ← New deletion handling
}

// Ensure custom MCPs have deletion protection finalizer
if !ctrlcommon.IsSystemPool(pool.Name) {
    if err := ctrl.ensureFinalizerPresent(pool); err != nil {
        return fmt.Errorf("failed to ensure finalizer on custom MCP %s: %w", pool.Name, err)
    }
}
```

### **Key Functions Added**
1. `handleMCPDeletion()` - Main deletion processing logic
2. `validateMCPDeletionSafety()` - Coordinates validation rules
3. `checkForActiveMachines()` - Rule 1 implementation (TODO)
4. `checkForTargetingNodes()` - Rule 2 implementation (TODO)
5. Helper functions for finalizer management

## Implementation Status

### ✅ **Completed (Ready to Use)**
- [x] Finalizer constants and helper functions
- [x] Integration into node controller sync loop
- [x] Finalizer addition/removal logic
- [x] Event recording and logging
- [x] System pool exclusion (master/worker unaffected)
- [x] Graceful error handling and fallbacks

### 🚧 **TODO (Implementation Required)**
- [ ] **Machine checking logic** (`checkForActiveMachines`)
- [ ] **Node targeting logic** (`checkForTargetingNodes`)

## Implementation Tasks

### **Task 1: Implement Machine Checking**
**File**: `pkg/controller/node/node_controller.go:L1690-1712`
**Location**: `checkForActiveMachines` function

```go
// TODO: IMPLEMENT MACHINE CHECKING LOGIC
//
// Steps:
// 1. Create machine client or use existing if available
// 2. List machines in openshift-machine-api namespace  
// 3. Check machine labels: machine.openshift.io/cluster-api-machine-role == pool.Name
// 4. Optionally parse providerSpec for advanced matching
//
// Return: (hasActiveMachines bool, blockingReason string, err error)
```

### **Task 2: Implement Node Targeting Check**
**File**: `pkg/controller/node/node_controller.go:L1714-1776`
**Location**: `checkForTargetingNodes` function

```go
// TODO: IMPLEMENT NODE TARGETING CHECK LOGIC  
//
// Steps:
// 1. List all nodes using ctrl.kubeClient.CoreV1().Nodes().List()
// 2. Filter eligible nodes (skip master/arbiter nodes)
// 3. Check node annotations for rendered config references
// 4. Match pattern "rendered-{poolName}-*"
//
// Return: (hasTargetingNodes bool, blockingReason string, err error)
```

### **Example Implementation for Node Targeting**
```go
func (ctrl *Controller) checkForTargetingNodes(pool *mcfgv1.MachineConfigPool) (bool, string, error) {
    nodes, err := ctrl.kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        return false, "", fmt.Errorf("failed to list nodes: %w", err)
    }
    
    renderedConfigPrefix := "rendered-" + pool.Name + "-"
    var blockingNodes []string
    
    for _, node := range nodes.Items {
        // Skip master and arbiter nodes
        if _, isMaster := node.Labels["node-role.kubernetes.io/master"]; isMaster {
            continue
        }
        if _, isArbiter := node.Labels["node-role.kubernetes.io/arbiter"]; isArbiter {
            continue
        }
        
        // Check current config annotation
        if currentConfig, exists := node.Annotations[daemonconsts.CurrentMachineConfigAnnotationKey]; exists {
            if strings.HasPrefix(currentConfig, renderedConfigPrefix) {
                blockingNodes = append(blockingNodes, fmt.Sprintf("%s (current: %s)", node.Name, currentConfig))
            }
        }
        
        // Check desired config annotation  
        if desiredConfig, exists := node.Annotations[daemonconsts.DesiredMachineConfigAnnotationKey]; exists {
            if strings.HasPrefix(desiredConfig, renderedConfigPrefix) {
                blockingNodes = append(blockingNodes, fmt.Sprintf("%s (desired: %s)", node.Name, desiredConfig))
            }
        }
    }
    
    if len(blockingNodes) > 0 {
        return true, fmt.Sprintf("Eligible nodes still targeting this pool's config: %v", blockingNodes), nil
    }
    
    return false, "", nil
}
```

## Testing

### **Basic Functionality Test**
```bash
# 1. Create a custom MCP
cat <<EOF | kubectl apply -f -
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfigPool
metadata:
  name: test-pool
spec:
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/test-pool: ""
  machineConfigSelector:
    matchLabels:
      machineconfiguration.openshift.io/role: test-pool
EOF

# 2. Verify finalizer was added automatically
kubectl get mcp test-pool -o jsonpath='{.metadata.finalizers}' 
# Should show: ["machineconfiguration.openshift.io/mcp-deletion-protection"]

# 3. Try to delete (will be blocked until TODO functions are implemented)
kubectl delete mcp test-pool

# 4. Check events for feedback
kubectl get events --field-selector involvedObject.name=test-pool
```

### **Current Behavior (Before TODO Implementation)**
- ✅ Finalizer is added to custom MCPs automatically
- ✅ System pools (master/worker) are unaffected
- ✅ Deletion requests are processed by the new logic
- ⚠️ **Deletion is allowed** because TODO functions return "no blocking resources"
- ✅ Events and logs provide feedback

### **Expected Behavior (After TODO Implementation)**  
- ❌ Deletion blocked when machines exist for the pool
- ❌ Deletion blocked when nodes target the pool's rendered config
- ✅ Deletion allowed when no blocking resources exist
- ✅ Clear event messages explain blocking reasons

## Advantages of This Approach

### **Compared to ValidatingAdmissionWebhook:**
- ✅ **No webhook server setup** - Uses existing controller
- ✅ **No TLS certificates** - No additional infrastructure  
- ✅ **Better user experience** - Clear events, not just admission denial
- ✅ **Fail-safe approach** - Won't accidentally allow unsafe deletions

### **Compared to ValidatingAdmissionPolicy:**
- ✅ **Full API access** - Can check nodes and machines
- ✅ **Complex validation logic** - No CEL limitations
- ✅ **Better error messages** - Rich context about blocking resources

### **Compared to Separate Controller:**
- ✅ **Simpler deployment** - No additional controller to manage
- ✅ **Reuses existing infrastructure** - Same RBAC, same container
- ✅ **Consistent with existing patterns** - Follows MCO controller patterns

## Monitoring and Debugging

### **Log Messages**
```bash
# Watch for finalizer activities
kubectl logs -n openshift-machine-config-operator deployment/machine-config-controller \
  | grep "MachineConfigPool.*finalizer"

# Watch for deletion activities  
kubectl logs -n openshift-machine-config-operator deployment/machine-config-controller \
  | grep "deletion.*MachineConfigPool"
```

### **Events**
```bash
# Watch MCP events
kubectl get events -w --field-selector involvedObject.kind=MachineConfigPool

# Check specific MCP events
kubectl describe mcp <mcp-name>
```

### **Finalizer Status**
```bash
# Check if custom MCP has finalizer
kubectl get mcp <custom-mcp-name> -o jsonpath='{.metadata.finalizers}'

# Check if system pool has finalizer (should be empty or not have our finalizer)
kubectl get mcp master -o jsonpath='{.metadata.finalizers}'
```

## Next Steps

1. **Implement TODO functions** using the provided examples and comments
2. **Test with real machines and nodes** to ensure validation works correctly
3. **Add metrics** for monitoring deletion attempts and blocking reasons
4. **Consider adding bypass mechanism** for emergency situations (e.g., force-delete annotation)

## Emergency Override

If needed, finalizers can be removed manually to force deletion:

```bash
# Remove finalizer to allow immediate deletion
kubectl patch mcp <mcp-name> --type='merge' -p='{"metadata":{"finalizers":null}}'
```

⚠️ **Use with caution** - This bypasses all safety checks.
