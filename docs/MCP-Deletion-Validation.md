# MachineConfigPool Deletion Validation

This document describes the validation mechanism that prevents unsafe deletion of custom MachineConfigPools (MCPs).

## Overview

The MCP deletion validation prevents users from accidentally deleting custom MachineConfigPools that still have:

1. **Active Machines**: Machines that are configured to create nodes for the MCP
2. **Target Nodes**: Eligible nodes (non-master, non-arbiter) that are currently using the MCP's rendered configuration

This protection helps prevent cluster instability and orphaned resources.

## Components

### 1. ValidatingAdmissionPolicy (Kubernetes 1.26+)

Located in `manifests/admission/validating-admission-policy.yaml`

- **Pros**: Declarative, no additional code required
- **Cons**: Limited access to cluster state, complex CEL expressions
- **Best for**: Clusters with ValidatingAdmissionPolicy support

### 2. ValidatingAdmissionWebhook (Recommended)

Located in `pkg/webhooks/mcp_deletion_webhook.go`

- **Pros**: Full API access, comprehensive validation logic, better error messages  
- **Cons**: Requires webhook server setup and TLS certificates
- **Best for**: Production deployments

## Validation Rules

### Protected Resources

The validation **ONLY applies to custom MCPs**, not default system pools:
- ✅ `master` pool - Always allowed to delete
- ✅ `worker` pool - Always allowed to delete  
- ❌ `custom-role` pool - Subject to validation

### Blocking Conditions

Custom MCP deletion is **BLOCKED** if either condition is true:

#### Condition 1: Active Machines
```bash
# Machines with labels targeting the MCP
machine.openshift.io/cluster-api-machine-role: custom-role
machine.openshift.io/cluster-api-machine-type: custom-role
```

#### Condition 2: Nodes Targeting MCP Configuration
```bash
# Eligible nodes (non-master, non-arbiter) with annotations:
machineconfiguration.openshift.io/currentConfig: rendered-custom-role-abc123
machineconfiguration.openshift.io/desiredConfig: rendered-custom-role-def456
```

## Deployment

### Option 1: ValidatingAdmissionPolicy

```bash
# Apply the policy (requires Kubernetes 1.26+)
kubectl apply -f manifests/admission/validating-admission-policy.yaml
```

### Option 2: ValidatingAdmissionWebhook

1. **Generate TLS Certificates**
```bash
# Create webhook certificates
kubectl create secret tls webhook-certs \
  --cert=tls.crt \
  --key=tls.key \
  -n openshift-machine-config-operator
```

2. **Apply Webhook Configuration**
```bash
kubectl apply -f manifests/admission/webhook-configuration.yaml
```

3. **Update Machine Config Controller**

Add to `cmd/machine-config-controller/start.go`:
```go
// After line 148, add webhook integration
webhookManager, err := ctrlcommon.NewWebhookManager(
    ctrlctx.ClientBuilder.KubeClientOrDie("mcp-deletion-webhook"),
    ctrlctx.ClientBuilder.MachineConfigClientOrDie("mcp-deletion-webhook"),  
    ctrlctx.ClientBuilder.MachineClientOrDie("mcp-deletion-webhook"),
)
if err != nil {
    klog.Fatalf("Failed to create webhook manager: %v", err)
}

go webhookManager.Start(ctx)
```

4. **Configure TLS Mount**

Update the Machine Config Controller deployment to mount certificates:
```yaml
spec:
  containers:
  - name: machine-config-controller
    volumeMounts:
    - name: webhook-certs
      mountPath: /etc/certs
      readOnly: true
    args:
    - start
    - --webhook-port=9443
    - --webhook-cert-path=/etc/certs/tls.crt
    - --webhook-key-path=/etc/certs/tls.key
  volumes:
  - name: webhook-certs
    secret:
      secretName: webhook-certs
```

## Usage Examples

### ✅ Allowed Deletions

```bash
# Delete default pools (always allowed)
kubectl delete mcp master
kubectl delete mcp worker

# Delete custom pool with no dependencies
kubectl delete mcp clean-custom-pool
```

### ❌ Blocked Deletions

```bash
# This will be blocked:
kubectl delete mcp custom-gpu-pool
# Error: Cannot delete custom MachineConfigPool 'custom-gpu-pool': 
# machines still exist in pool

# This will also be blocked:
kubectl delete mcp custom-role
# Error: Cannot delete custom MachineConfigPool 'custom-role':
# nodes still targeting pool configuration
```

## Safe Deletion Process

To safely delete a custom MCP:

1. **Scale down machines**
```bash
# Remove machines targeting the MCP
oc scale machineset custom-role-machineset --replicas=0
```

2. **Wait for nodes to drain**
```bash
# Monitor nodes moving to different pools
oc get nodes --show-labels
```

3. **Verify clean state**
```bash
# Check no machines target the pool
oc get machines -o yaml | grep -i custom-role

# Check no nodes target the rendered config  
oc get nodes -o yaml | grep "rendered-custom-role"
```

4. **Delete the MCP**
```bash
kubectl delete mcp custom-role
```

## Troubleshooting

### Check Webhook Status
```bash
# Verify webhook is running
oc get pods -n openshift-machine-config-operator
oc logs -n openshift-machine-config-operator deployment/machine-config-controller

# Check webhook configuration
oc get validatingadmissionwebhook mcp-deletion-validator -o yaml
```

### Debug Blocked Deletions
```bash
# Find machines targeting the MCP
oc get machines -o json | jq '.items[] | select(.metadata.labels."machine.openshift.io/cluster-api-machine-role" == "custom-role")'

# Find nodes with MCP annotations
oc get nodes -o json | jq '.items[] | select(.metadata.annotations."machineconfiguration.openshift.io/currentConfig" | startswith("rendered-custom-role"))'
```

### Bypass Protection (Emergency)

In emergency situations, you can temporarily disable validation:

```bash
# Delete the webhook configuration
kubectl delete validatingadmissionwebhook mcp-deletion-validator

# Or delete the admission policy
kubectl delete validatingadmissionpolicy block-custom-mcp-deletion
kubectl delete validatingadmissionpolicybinding block-custom-mcp-deletion-binding
```

## Testing

Run the webhook tests:
```bash
cd pkg/webhooks
go test -v ./...
```

## Security Considerations

- The webhook requires `get`, `list`, `watch` permissions on `nodes`, `machines`, and `machineconfigpools`
- TLS certificates should be rotated regularly
- Webhook failures block ALL MCP deletions (fail-closed behavior)
- Consider monitoring webhook availability and performance

## Related Issues

- [MCP deletion cleanup](https://github.com/openshift/machine-config-operator/issues/301)
- [Rendered MC garbage collection](https://github.com/openshift/machine-config-operator/pull/318)
