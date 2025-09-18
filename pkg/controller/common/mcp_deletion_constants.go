package common

const (
	// MCPDeletionFinalizerPrefix is the finalizer added to custom MCPs to prevent unsafe deletion
	MCPDeletionFinalizerPrefix = "machineconfiguration.openshift.io/mcp-deletion-protection"
)

// GetMCPDeletionFinalizer returns the finalizer name for the given MCP
func GetMCPDeletionFinalizer(mcpName string) string {
	return MCPDeletionFinalizerPrefix + "-" + mcpName
}
