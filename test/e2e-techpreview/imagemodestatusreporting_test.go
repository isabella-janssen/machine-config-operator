package e2e_techpreview_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	testHelpers "github.com/openshift/machine-config-operator/test/helpers"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/openshift/machine-config-operator/test/helpers"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog/v2"

	ign3types "github.com/coreos/ignition/v2/config/v3_5/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// The custom MachineConfigPool name.
	customMCPName string = "infra"

	// The worker MachineConfigPool name.
	workerMCPName string = "worker"
)

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

// ****Helpers start here****
// `createMCForTest` creates a MachineConfig to be used in an e2e test. The resulting MC will target
// the desired MCP and will be a valid if the input `isValidMC` value is true, or invalid otherwise.
func createMCForTest(mcpName string, isValidMC bool) *mcfgv1.MachineConfig {
	mcName := "99-worker-valid-testfile"
	fileName := "/home/core/test"
	if !isValidMC {
		mcName = "99-worker-invalid-testfile"
		fileName = "/home/core"
	}
	mc := testHelpers.CreateMC(mcName, mcpName)

	ignConfig := ctrlcommon.NewIgnConfig()
	ignFile := testHelpers.CreateUncompressedIgn3File(fileName, "data:,hello-world", 420)
	ignConfig.Storage.Files = append(ignConfig.Storage.Files, ignFile)
	rawIgnConfig := testHelpers.MarshalOrDie(ignConfig)
	mc.Spec.Config.Raw = rawIgnConfig

	return mc
}

func createMC(mcpName string, isValidMC bool) *mcfgv1.MachineConfig {
	mcName := fmt.Sprintf("%s-%s", mcpName, uuid.NewUUID())
	filePath := "/core/test"
	if !isValidMC {
		filePath = "/core"
	}

	mc := helpers.NewMachineConfig(mcName, helpers.MCLabelForRole(workerMCPName), "", []ign3types.File{
		helpers.CreateEncodedIgn3File(filepath.Join("/home", filePath), mcName, 420),
	})

	klog.Error(mc)
	return mc
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
		if condition.Type == mcfgv1.MachineConfigPoolUpdated && condition.Status == corev1.ConditionTrue {
			isUpdated = true
		} else if condition.Type == mcfgv1.MachineConfigPoolUpdating && condition.Status == corev1.ConditionTrue {
			isUpdating = true
		}

		if condition.Type == mcfgv1.MachineConfigPoolDegraded && condition.Status == corev1.ConditionTrue {
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

// ****Helpers end here****

// // `TestMachineCountOnUpdateCustomMCP` tests that the MCP updating and updated machine counts are
// // correctly populated on a node update in a custom MCP.
// func TestMachineCountOnUpdateCustomMCP(t *testing.T) {
// 	// Create client set for test
// 	cs := framework.NewClientSet("")

// 	// Get all worker nodes
// 	allWorkerNodes, nodeErr := helpers.GetNodesByRole(cs, workerMCPName)
// 	require.NoError(t, nodeErr, fmt.Sprintf("Error getting nodes from `%v` MCP.", workerMCPName))

// 	// Get starting machine counts for default `worker` MCP
// 	startingWorkerMachineCount, _, countErr := getMachineCountsAndMCPState(cs, workerMCPName)
// 	require.NoError(t, countErr)
// 	t.Logf("Starting test with %v `%v` nodes.", startingWorkerMachineCount.totalMachineCount, workerMCPName)

// 	// Create a custom MCP and assign a worker node to it
// 	customNode := helpers.GetRandomNode(t, cs, "worker")
// 	helpers.CreatePoolWithNode(t, cs, customMCPName, customNode)

// 	// TODO: in origin, check machine count throughout mcp mc apply, updated with 0 machines, add node, updating with 0 updated, one updating, etc.

// 	// Validate the infra and worker MCP machine counts
// 	workerMachineCount, workerMachineStatus, countErr := getMachineCountsAndMCPState(cs, workerMCPName)
// 	require.NoError(t, countErr, "Error getting machine information for worker MCP.")
// 	customMachineCount, customMachineStatus, countErr := getMachineCountsAndMCPState(cs, workerMCPName)
// 	require.NoError(t, countErr, "Error getting machine information for infra MCP.")
// 	require.Equal(t, workerMachineCount.updatedMachineCount, startingWorkerMachineCount.totalMachineCount-1, fmt.Sprintf("`%v` MCP has %v updated machines, expected %v.", workerMCPName, workerMachineCount.updatedMachineCount, startingWorkerMachineCount.totalMachineCount-1))
// 	require.True(t, workerMachineStatus.isUpdated, fmt.Sprintf("`%v` MCP is not in 'Updated' state. Degraded: %v, Updating: %v", workerMCPName, workerMachineStatus.isDegraded, workerMachineStatus.isUpdating))
// 	require.Equal(t, customMachineCount.updatedMachineCount, 1, fmt.Sprintf("`%v` MCP has %v updated machines, expected %v.", customMCPName, customMachineCount.updatedMachineCount, 1))
// 	require.True(t, customMachineStatus.isUpdated, fmt.Sprintf("`%v` MCP is not in 'Updated' state. Degraded: %v, Updating: %v", customMCPName, customMachineStatus.isDegraded, customMachineStatus.isUpdating))

// 	workerMCP, mcpErr := cs.MachineConfigPools().Get(context.TODO(), workerMCPName, metav1.GetOptions{})
// 	require.NoError(t, mcpErr, fmt.Sprintf("Error getting `%v` MCP.", workerMCPName))
// 	expectedWorkerUpdatingCount = ctrlcommon.GetUpdatedMachines(workerMCP, allWorkerNodes, &mcfgv1.MachineOSConfig{}, &mcfgv1.MachineOSBuild{}, false)

// 	// check machine counts throughotu here
// 	// once infra is created it'll be updated with 0 machines
// 	// on label infra should flip to havine one upating machine
// 	// on label worker should flip to 2 updated machines and stay updated
// 	// when infra is updated it should have one ready node

// 	// Create & apply a MachineConfig to the custom MCP
// 	mc := createMCForTest(customMCPName, true)
// 	// TODO: remove post testing
// 	t.Logf("MC: %v", mc)
// 	mcCleanupFunc := helpers.ApplyMC(t, cs, mc)

// 	// machineCounts, mcpStatuses, := getMachineCount(cs, customMCPName)

// 	// While the node is updating, check that the MCP machines are updated correctly

// 	// When updated, make sure 1 updated machine

// 	// When returning to normal, check machine coutns

// 	// TODO: update this!
// 	t.Cleanup(func() {
// 		// Unlabel node
// 		// wait for mcp to return to normal
// 		mcCleanupFunc()
// 	})

// 	customNodeMCN, customErr := cs.MachineconfigurationV1Interface.MachineConfigNodes().Get(context.TODO(), customNode.Name, metav1.GetOptions{})
// 	require.Equal(t, customMCPName, customNodeMCN.Spec.Pool.Name)
// 	require.NoError(t, customErr)
// }

// // cleanup

// // degrade --> fix degrade --> track counts

// // test pause case

// // update code flow to not have conditional check for updating and updated being right
// // Update test to compare MCP counts to the counts from the previously existing get XX machine count

// `TestMachineCountOnUpdateDefaultMCP` tests that the MCP updating and updated machine counts are
// correctly populated on a node update in a custom MCP.
// func TestMachineCountOnUpdateDefaultMCP(t *testing.T) {
// 	// Create client set for test
// 	cs := framework.NewClientSet("")

// 	// Create a valid MachineConfig targeting the default worker pool & apply it
// 	// mc := createMCForTest(workerMCPName, true)
// 	// mcName := fmt.Sprintf("%s-%s", workerMCPName, uuid.NewUUID())
// 	mc := createMC(workerMCPName, true)

// 	// // mc := helpers.NewMachineConfig(mcName, helpers.MCLabelForRole(workerMCPName), "", []ign3types.File{
// 	// // 	helpers.CreateEncodedIgn3File(filepath.Join("/etc", name), name, 420),
// 	// // })
// 	// t.Log(mc.Spec)
// 	// // t.Logf("here")
// 	// // t.Logf("MC: %v", mc) // TODO: remove post testing
// 	mcCleanupFunc := helpers.ApplyMC(t, cs, mc)
// 	// t.Logf("here2")

// 	// // Once the first build has started, we create a new MachineConfig, wait for
// 	// // the rendered config to appear, then we check that a new MachineOSBuild has
// 	// // started for that new MachineConfig.
// 	// mcName := "new-machineconfig"
// 	// mc := newMachineConfigTriggersImageRebuild(mcName, layeredMCPName, []string{"usbguard"})
// 	// applyMC(t, cs, mc)

// 	// Throughout a 7 minute period, check every 30 seconds that the updating and updated machine
// 	// count populated in the MCP matches the values calculated using historical functions.
// 	// TODO: evaluate the poke timing
// 	ctx := context.TODO()
// 	var errors []string
// 	wait.PollUntilContextTimeout(ctx, 30*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
// 		// Get `worker` MCP
// 		workerMcp, mcpErr := cs.MachineConfigPools().Get(ctx, workerMCPName, metav1.GetOptions{})
// 		if mcpErr != nil {
// 			t.Errorf("Error getting `%v` MCP.", workerMCPName)
// 			return false, mcpErr
// 		}

// 		// Get expected MCP machine counts from node annotations
// 		workerNodes, nodeErr := helpers.GetNodesByRole(cs, workerMCPName)
// 		if nodeErr != nil {
// 			t.Errorf("Error getting nodes from `%v` MCP.", workerMCPName)
// 			return false, nodeErr
// 		}
// 		var workerNodesFormatted []*corev1.Node
// 		for _, node := range workerNodes {
// 			workerNodesFormatted = append(workerNodesFormatted, &node)
// 		}
// 		expectedUpdatedWorkerMachines := ctrlcommon.GetUpdatedMachines(workerMcp, workerNodesFormatted, &mcfgv1.MachineOSConfig{}, &mcfgv1.MachineOSBuild{}, false)
// 		expectedDegradedWorkerMachines := ctrlcommon.GetDegradedMachines(workerNodesFormatted)

// 		// Ensure the MCP machine counts match the expected counts
// 		if workerMcp.Status.UpdatedMachineCount != int32(len(expectedUpdatedWorkerMachines)) {
// 			errors = append(errors, fmt.Sprintf("%v: Updated machine count is %v, not the expected value of %v.", time.Now(), workerMcp.Status.UpdatedMachineCount, len(expectedUpdatedWorkerMachines)))
// 		}
// 		if workerMcp.Status.DegradedMachineCount != int32(len(expectedDegradedWorkerMachines)) {
// 			errors = append(errors, fmt.Sprintf("%v: Degraded machine count is %v, not the expected value of %v", time.Now(), workerMcp.Status.DegradedMachineCount, len(expectedDegradedWorkerMachines)))
// 		}

// 		return false, nil
// 	})

// 	// Delete the applied MC then throughout a 7 minute period or until the pool returns to an
// 	// updated state, check every 30 seconds that the updating and updated machine count populated
// 	//  in the MCP matches the values calculated using historical functions.
// 	mcCleanupFunc()
// 	wait.PollUntilContextTimeout(ctx, 30*time.Second, 7*time.Minute, true, func(ctx context.Context) (bool, error) {
// 		// Get `worker` MCP
// 		workerMcp, mcpErr := cs.MachineConfigPools().Get(context.TODO(), workerMCPName, metav1.GetOptions{})
// 		if mcpErr != nil {
// 			t.Errorf("Error getting `%v` MCP.", workerMCPName)
// 			return false, mcpErr
// 		}

// 		// Get expected MCP machine counts from node annotations
// 		workerNodes, nodeErr := helpers.GetNodesByRole(cs, workerMCPName)
// 		if nodeErr != nil {
// 			t.Errorf("Error getting nodes from `%v` MCP.", workerMCPName)
// 			return false, nodeErr
// 		}
// 		var workerNodesFormatted []*corev1.Node
// 		for _, node := range workerNodes {
// 			workerNodesFormatted = append(workerNodesFormatted, &node)
// 		}
// 		expectedUpdatedWorkerMachines := ctrlcommon.GetUpdatedMachines(workerMcp, workerNodesFormatted, &mcfgv1.MachineOSConfig{}, &mcfgv1.MachineOSBuild{}, false)
// 		expectedDegradedWorkerMachines := ctrlcommon.GetDegradedMachines(workerNodesFormatted)

// 		// Ensure the MCP machine counts match the expected counts
// 		if workerMcp.Status.UpdatedMachineCount != int32(len(expectedUpdatedWorkerMachines)) {
// 			errors = append(errors, fmt.Sprintf("%v: Updated machine count is %v, not the expected value of %v.", time.Now(), workerMcp.Status.UpdatedMachineCount, len(expectedUpdatedWorkerMachines)))
// 		}
// 		if workerMcp.Status.DegradedMachineCount != int32(len(expectedDegradedWorkerMachines)) {
// 			errors = append(errors, fmt.Sprintf("%v: Degraded machine count is %v, not the expected value of %v", time.Now(), workerMcp.Status.DegradedMachineCount, len(expectedDegradedWorkerMachines)))
// 		}

// 		// End polling if MCP is returned to an Updated state
// 		for _, condition := range workerMcp.Status.Conditions {
// 			if condition.Type == mcfgv1.MachineConfigPoolUpdated {
// 				return condition.Status == corev1.ConditionTrue, nil
// 			}
// 		}

// 		return false, nil
// 	})

// 	// TODO: in origin, check machine count throughout mcp mc apply, updated with 0 machines, add node, updating with 0 updated, one updating, etc.

// 	// check machine counts throughotu here
// 	// once infra is created it'll be updated with 0 machines
// 	// on label infra should flip to havine one upating machine
// 	// on label worker should flip to 2 updated machines and stay updated
// 	// when infra is updated it should have one ready node

// 	// machineCounts, mcpStatuses, := getMachineCount(cs, customMCPName)

// 	// While the node is updating, check that the MCP machines are updated correctly

// 	// When updated, make sure 1 updated machine

// 	// When returning to normal, check machine coutns

// 	// TODO: update this!
// 	t.Cleanup(func() {
// 		// Unlabel node
// 		// wait for mcp to return to normal
// 		mcCleanupFunc()
// 	})
// }

func TestMCApply(t *testing.T) {
	cs := framework.NewClientSet("")

	mcName := "new-machineconfig"
	mc := newMachineConfigTriggersImageRebuild(mcName, workerMCPName, []string{"usbguard"})
	applyMC(t, cs, mc)

	_, err := helpers.WaitForRenderedConfig(t, cs, workerMCPName, mcName)
	require.NoError(t, err)
}
