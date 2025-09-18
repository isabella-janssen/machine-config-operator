package common

const (
	// MCPDeletionFinalizer is the finalizer added to custom MCPs to prevent unsafe deletion
	MCPDeletionFinalizer = "machineconfiguration.openshift.io/mcp-deletion-protection"
)

// IsSystemPool returns true if the pool is a system pool (master or worker)
func IsSystemPool(poolName string) bool {
	return poolName == "master" || poolName == "worker"
}
