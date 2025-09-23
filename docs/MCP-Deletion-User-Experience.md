# MachineConfigPool Deletion User Experience

This document explains what happens when you delete a custom MachineConfigPool and how to monitor/troubleshoot the process.

## Understanding the Deletion Process

When you run `oc delete mcp/custom-pool`, here's what happens:

### 1. **Immediate Response**
```bash
$ oc delete mcp/infra
machineconfigpool.machineconfiguration.openshift.io "infra" deleted
```

⚠️ **The "deleted" message appears immediately, but this does NOT mean the MCP is actually deleted yet!**

### 2. **Background Validation**
- Kubernetes sets a `deletionTimestamp` on the MCP
- The node controller validates if deletion is safe
- If **unsafe**: The command hangs while the finalizer prevents actual deletion
- If **safe**: The finalizer is removed and deletion completes immediately

## Why the Command Hangs

The `oc delete` command hangs because:

1. **Finalizer Protection**: Custom MCPs have a finalizer that blocks deletion
2. **Safety Validation**: The controller checks for blocking conditions
3. **Pending State**: The MCP remains in "Terminating" state until validation passes

## Monitoring the Deletion Process

### **Check Events (Recommended)**
```bash
# Check events for your specific MCP
oc get events --field-selector involvedObject.name=infra

# Watch events in real-time  
oc get events -w --field-selector involvedObject.name=infra
```

**Example Output:**
```
LAST SEEN   TYPE      REASON            OBJECT                   MESSAGE
2m          Warning   DeletionBlocked   machineconfigpool/infra  MachineConfigPool infra cannot be deleted: Eligible nodes still targeting this pool's config: [worker-1, worker-2]. To fix: remove the node-role label or wait for nodes to transition...
```

### **Check MCP Status Conditions**
```bash
# Check the MCP status for deletion conditions
oc get mcp infra -o yaml | grep -A10 conditions

# Or use describe for better formatting
oc describe mcp infra
```

**Look for:**
```yaml
conditions:
- type: DeletionBlocked
  status: "True"  
  reason: UnsafeToDelete
  message: "Cannot delete MachineConfigPool: Eligible nodes still targeting..."
```

### **Check Controller Logs**
```bash
# Check machine-config-controller logs
oc logs -n openshift-machine-config-operator deployment/machine-config-controller | grep infra

# Follow logs in real-time
oc logs -f -n openshift-machine-config-operator deployment/machine-config-controller | grep "DELETION BLOCKED\|User action required"
```

**Example Log Output:**
```
DELETION BLOCKED: MachineConfigPool infra cannot be deleted: Eligible nodes still targeting this pool's config: [worker-1, worker-2]
User action required: The 'oc delete mcp infra' command will remain pending until blocking conditions are resolved
```

## Resolving Blocking Conditions

### **Condition 1: Active Machines**
**Problem**: `MachineConfigPool still has 2 machines according to status`

**Solutions**:
```bash
# Option A: Scale down the machines
oc get machines -n openshift-machine-api | grep infra
oc delete machine <machine-name> -n openshift-machine-api

# Option B: Change machine role labels to target different pool  
oc patch machine <machine-name> -n openshift-machine-api --type='merge' -p='{"metadata":{"labels":{"machine.openshift.io/cluster-api-machine-role":"worker"}}}'
```

### **Condition 2: Nodes Targeting Pool's Config**  
**Problem**: `Eligible nodes still targeting this pool's config: [worker-1, worker-2]`

**Solutions**:
```bash
# Option A: Remove the node role label
oc label node worker-1 node-role.kubernetes.io/infra-
oc label node worker-2 node-role.kubernetes.io/infra-

# Option B: Wait for nodes to transition to different config
# (This happens automatically if you've already changed their role)

# Option C: Add a different role label
oc label node worker-1 node-role.kubernetes.io/worker=
oc label node worker-2 node-role.kubernetes.io/worker=
```

## Canceling a Stuck Deletion

If you need to cancel a hanging deletion:

### **Option 1: Fix the Blocking Condition (Recommended)**
Follow the resolution steps above to address the actual blocking condition.

### **Option 2: Force Deletion (Emergency Only)**
⚠️ **This bypasses all safety checks - use only if absolutely necessary!**

```bash
# Remove the finalizer manually to force deletion
oc patch mcp infra --type='json' -p='[{"op": "remove", "path": "/metadata/finalizers"}]'
```

## Complete Example Workflow

```bash
# 1. Attempt deletion
$ oc delete mcp/infra
machineconfigpool.machineconfiguration.openshift.io "infra" deleted
# ← Command hangs here

# 2. In another terminal, check why it's blocked
$ oc get events --field-selector involvedObject.name=infra
LAST SEEN   TYPE      REASON            OBJECT                   MESSAGE
1m          Warning   DeletionBlocked   machineconfigpool/infra  MachineConfigPool infra cannot be deleted: Eligible nodes still targeting this pool's config: [worker-1, worker-2]

# 3. Fix the blocking condition  
$ oc label node worker-1 node-role.kubernetes.io/infra-
$ oc label node worker-2 node-role.kubernetes.io/infra-

# 4. Deletion completes automatically
# ← The hanging 'oc delete' command now finishes
```

## Tips for Better Experience

### **1. Prepare Before Deletion**
```bash
# Check what would block deletion BEFORE running delete
oc get mcp infra -o yaml | grep machineCount
oc get nodes -l node-role.kubernetes.io/infra
```

### **2. Use Describe for Status**  
```bash
# Get comprehensive status including conditions
oc describe mcp infra
```

### **3. Monitor in Parallel**
```bash
# Terminal 1: Run the deletion
oc delete mcp/infra

# Terminal 2: Watch events  
oc get events -w --field-selector involvedObject.name=infra

# Terminal 3: Monitor logs
oc logs -f -n openshift-machine-config-operator deployment/machine-config-controller | grep infra
```

## System Pools Behavior

**System pools are NOT protected** and delete immediately:

```bash
# These work immediately (no finalizer protection)
oc delete mcp/master   # ✅ Deletes immediately  
oc delete mcp/worker   # ✅ Deletes immediately
oc delete mcp/arbiter  # ✅ Deletes immediately

# Custom pools have protection  
oc delete mcp/infra    # ⚠️ Validated before deletion
oc delete mcp/gpu      # ⚠️ Validated before deletion  
```

## Troubleshooting

### **"Command hangs with no events"**
- Check controller logs for errors
- Verify the machine-config-controller is running
- Check if the MCP actually has the finalizer: `oc get mcp <name> -o yaml | grep finalizers`

### **"Events show 'DeletionAllowed' but still hangs"**
- There might be other finalizers on the MCP
- Check: `oc get mcp <name> -o yaml | grep -A5 finalizers`

### **"Want to skip validation entirely"**
- Use the force deletion approach above (emergency only)
- Or disable the feature entirely by modifying the controller code

This enhanced user experience provides clear feedback about what's happening and how to resolve blocking conditions! 🚀

