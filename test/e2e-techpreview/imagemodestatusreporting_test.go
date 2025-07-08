package e2e_techpreview_test

import (
	"context"
	"fmt"
	"testing"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/openshift/machine-config-operator/test/helpers"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// The custom MachineConfigPool name.
	customMCPName string = "infra"

	// The worker MachineConfigPool name.
	workerMCPName string = "worker"
)

// `createMCForTest` creates a MachineConfig to be used in an e2e test. The resulting MC will target
// the desired MCP and will be a valid if the input `isValidMC` value is true, or invalid otherwise.
func createMCForTest(mcpName string, isValidMC bool) *mcfgv1.MachineConfig {
	mcName := "99-worker-valid-testfile"
	fileName := "/home/core/test"
	if !isValidMC {
		mcName = "99-worker-invalid-testfile"
		fileName = "/home/core"
	}
	mc := helpers.CreateMC(mcName, mcpName)

	ignConfig := ctrlcommon.NewIgnConfig()
	ignFile := helpers.CreateUncompressedIgn3File(fileName, "data:,hello-world", 420)
	ignConfig.Storage.Files = append(ignConfig.Storage.Files, ignFile)
	rawIgnConfig := helpers.MarshalOrDie(ignConfig)
	mc.Spec.Config.Raw = rawIgnConfig

	return mc
}

// `TestMachineCountOnUpdateCustomMCP` tests that the MCP updating and updated machine counts are
// correctly populated on a node update in a custom MCP.
func TestMachineCountOnUpdateCustomMCP(t *testing.T) {
	// Create client set for test
	cs := framework.NewClientSet("")

	// Get starting machine counts for default `worker` MCP
	startingWorkerMachineCount, _, countErr := getMachineCountsAndMCPState(cs, workerMCPName)
	require.NoError(t, countErr)
	t.Logf("Starting test with %v `%v` nodes.", startingWorkerMachineCount.totalMachineCount, workerMCPName)

	// Create a custom MCP and assign a worker node to it
	customNode := helpers.GetRandomNode(t, cs, "worker")
	helpers.CreatePoolWithNode(t, cs, customMCPName, customNode)

	// TODO: in origin, check machine count throughout mcp mc apply, updated with 0 machines, add node, updating with 0 updated, one updating, etc.

	// Validate the infra and worker MCP machine counts
	workerMachineCount, workerMachineStatus, countErr := getMachineCountsAndMCPState(cs, workerMCPName)
	require.NoError(t, countErr, "Error getting machine information for worker MCP.")
	customMachineCount, customMachineStatus, countErr := getMachineCountsAndMCPState(cs, workerMCPName)
	require.NoError(t, countErr, "Error getting machine information for infra MCP.")
	require.Equal(t, workerMachineCount.updatedMachineCount, startingWorkerMachineCount.totalMachineCount-1, fmt.Sprintf("`%v` MCP has %v updated machines, expected %v.", workerMCPName, workerMachineCount.updatedMachineCount, startingWorkerMachineCount.totalMachineCount-1))
	require.True(t, workerMachineStatus.isUpdated, fmt.Sprintf("`%v` MCP is not in 'Updated' state. Degraded: %v, Updating: %v", workerMCPName, workerMachineStatus.isDegraded, workerMachineStatus.isUpdating))
	require.Equal(t, customMachineCount.updatedMachineCount, 1, fmt.Sprintf("`%v` MCP has %v updated machines, expected %v.", customMCPName, customMachineCount.updatedMachineCount, 1))
	require.True(t, customMachineStatus.isUpdated, fmt.Sprintf("`%v` MCP is not in 'Updated' state. Degraded: %v, Updating: %v", customMCPName, customMachineStatus.isDegraded, customMachineStatus.isUpdating))

	// check machine counts throughotu here
	// once infra is created it'll be updated with 0 machines
	// on label infra should flip to havine one upating machine
	// on label worker should flip to 2 updated machines and stay updated
	// when infra is updated it should have one ready node

	// Create & apply a MachineConfig to the custom MCP
	mc := createMCForTest(customMCPName, true)
	// TODO: remove post testing
	t.Logf("MC: %v", mc)
	mcCleanupFunc := helpers.ApplyMC(t, cs, mc)

	// machineCounts, mcpStatuses, := getMachineCount(cs, customMCPName)

	// While the node is updating, check that the MCP machines are updated correctly

	// When updated, make sure 1 updated machine

	// When returning to normal, check machine coutns

	// TODO: update this!
	t.Cleanup(func() {
		// Unlabel node
		// wait for mcp to return to normal
		mcCleanupFunc()
	})

	customNodeMCN, customErr := cs.MachineconfigurationV1Interface.MachineConfigNodes().Get(context.TODO(), customNode.Name, metav1.GetOptions{})
	require.Equal(t, customMCPName, customNodeMCN.Spec.Pool.Name)
	require.NoError(t, customErr)
}

// cleanup

// degrade --> fix degrade --> track counts

type MachineCounts struct {
	totalMachineCount    int
	updatedMachineCount  int
	degradedMachineCount int
}

type MachineStatuses struct {
	isUpdated  bool
	isUpdating bool
	isDegraded bool
}

// `getMachineCountsAndMCPState` gets the total, updated, and degraded machine counts and the
// condition statuses of an MCP.
func getMachineCountsAndMCPState(cs *framework.ClientSet, mcpName string) (MachineCounts, MachineStatuses, error) {
	// Get the desried MCP
	mcp, mcpErr := cs.MachineConfigPools().Get(context.TODO(), mcpName, metav1.GetOptions{})
	if mcpErr != nil {
		return MachineCounts{}, MachineStatuses{}, mcpErr
	}

	// Get the condition statuses
	isUpdated := false
	isUpdating := false
	isDegraded := false
	for _, condition := range mcp.Status.Conditions {
		if condition.Type == mcfgv1.MachineConfigPoolUpdated && condition.Status == v1.ConditionTrue {
			isUpdated = true
		} else if condition.Type == mcfgv1.MachineConfigPoolUpdating && condition.Status == v1.ConditionTrue {
			isUpdating = true
		}

		if condition.Type == mcfgv1.MachineConfigPoolDegraded && condition.Status == v1.ConditionTrue {
			isDegraded = true
		}
	}

	return MachineCounts{
			totalMachineCount:    int(mcp.Status.MachineCount),
			updatedMachineCount:  int(mcp.Status.UpdatedMachineCount),
			degradedMachineCount: int(mcp.Status.DegradedMachineCount),
		}, MachineStatuses{
			isUpdated,
			isUpdating,
			isDegraded,
		}, nil
}

// test pause case
