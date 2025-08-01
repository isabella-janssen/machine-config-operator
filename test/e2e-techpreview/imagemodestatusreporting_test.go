package e2e_techpreview_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"

	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/openshift/machine-config-operator/test/helpers"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// The worker MachineConfigPool name.
	workerMCPName string = "worker"

	// String for setting pretty timestamp format of `YYYY-MM-DD HH:MM:SS`
	timeFmtStr string = "2001-01-01 01:01:01"
)

// Struct for defining MCP machine counts
type MachineCount struct {
	updatedMachineCount  int32
	degradedMachineCount int32
}

// `TestMachineCountOnUpdateDefaultMCP` tests that the MCP updating and updated machine counts are
// correctly populated on a node update in a custom MCP.
func TestMachineCountOnUpdateDefaultMCP(t *testing.T) {
	// Create client set & context for test
	cs := framework.NewClientSet("")
	ctx := context.TODO()

	// Get starting `worker` config version
	workerMcp, mcpErr := cs.MachineConfigPools().Get(ctx, workerMCPName, metav1.GetOptions{})
	require.Nil(t, mcpErr, fmt.Sprintf("Error getting `%v` MCP: %v", workerMCPName, mcpErr))
	workerStartingConfigVersion := workerMcp.Spec.Configuration.Name
	t.Logf("workerStartingConfigVersion: %v", workerStartingConfigVersion)
	workerStartingMachineCount := workerMcp.Status.MachineCount

	// Create a valid MachineConfig targeting the default worker pool & apply it
	mc := createMCForTest(workerMCPName, true)
	t.Logf("Created MC: %v", mc.Name)
	mcCleanupFunc := applyMC(t, cs, mc)
	t.Logf("Applyed MC: %v", mc.Name)

	// Throughout a 7 minute period, check every 30 seconds that the updated and degraded machine
	// counts populated in the MCP match the values calculated using the historical functions.
	// TODO: evaluate the poke timing
	sleepInterval := 3  // number of seconds to sleep between MCP pokes
	totalTime := 3 * 60 // total number of seconds to poke MCP
	t.Log("Testing updated and degraded machine counts throughout MC apply.")
	var expectedUpdatedWorkerMachineCount, expectedDegradedWorkerMachineCount int32
	updating := false
	updateComplete := false
	for i := 0; i < totalTime/sleepInterval; i++ {
		// Get `worker` MCP
		workerMcp, mcpErr = cs.MachineConfigPools().Get(ctx, workerMCPName, metav1.GetOptions{})
		require.Nil(t, mcpErr, fmt.Sprintf("Error getting `%v` MCP: %v", workerMCPName, mcpErr))

		// Get expected MCP machine counts
		for _, condition := range workerMcp.Status.Conditions {
			// Determine if MCP is "Updating" or "Updated"
			if condition.Type == mcfgv1.MachineConfigPoolUpdated {
				if condition.Status == corev1.ConditionTrue { // Handle case when MCP is `Updated`
					t.Logf("%v: MCP %v is in `Updated` state.", time.Now().Format(timeFmtStr), workerMCPName)
					// updateComplete = true
					expectedUpdatedWorkerMachineCount = workerStartingMachineCount
					expectedDegradedWorkerMachineCount = 0
					// When the configuration version in the MCP status is no longer equal to the
					// original configuration version, the MCP is updated to the new MC
					if workerMcp.Status.Configuration.Name != workerStartingConfigVersion {
						// if updating { // If we get to a stage when the MCP status is updated and we have gone through the "updating" stage, we have fully completed the update
						t.Logf("%v: MCP %v is fully `Updated` to new desired config `%v`.", time.Now().Format(timeFmtStr), workerMCPName, workerMcp.Spec.Configuration.Name)
						updateComplete = true
						updating = false
					}
				} else {
					t.Logf("%v: MCP %v is updating.", time.Now().Format(timeFmtStr), workerMCPName)
					updating = true
				}
				break
			} else if condition.Type == mcfgv1.MachineConfigPoolUpdating && condition.Status == corev1.ConditionTrue {
				t.Logf("%v: MCP %v is starting to update.", time.Now().Format(timeFmtStr), workerMCPName)
				updating = true
				break
			}
		}

		// Handle case when MCP is updating by getting the expected machine counts from node annotations
		if updating {
			workerNodes, nodeErr := helpers.GetNodesByRole(cs, workerMCPName)
			require.Nil(t, nodeErr, fmt.Sprintf("Error getting nodes from `%v` MCP: %v", workerMCPName, nodeErr))
			var workerNodesFormatted []*corev1.Node
			for _, node := range workerNodes {
				workerNodesFormatted = append(workerNodesFormatted, &node)
			}
			expectedUpdatedWorkerMachines := ctrlcommon.GetUpdatedMachines(workerMcp, workerNodesFormatted, &mcfgv1.MachineOSConfig{}, &mcfgv1.MachineOSBuild{}, false)
			expectedUpdatedWorkerMachineCount = int32(len(expectedUpdatedWorkerMachines))
			expectedDegradedWorkerMachines := ctrlcommon.GetDegradedMachines(workerNodesFormatted)
			expectedDegradedWorkerMachineCount = int32(len(expectedDegradedWorkerMachines))
		}

		// Ensure the MCP updated and degraded machine counts match the expected counts
		require.True(t, workerMcp.Status.UpdatedMachineCount == expectedUpdatedWorkerMachineCount, fmt.Sprintf("Updated machine count is %v, not the expected value of %v. %v, ", workerMcp.Status.UpdatedMachineCount, expectedUpdatedWorkerMachineCount, workerMcp.Status))
		require.True(t, workerMcp.Status.DegradedMachineCount == expectedDegradedWorkerMachineCount, fmt.Sprintf("Degraded machine count is %v, not the expected value of %v", workerMcp.Status.DegradedMachineCount, expectedDegradedWorkerMachineCount))

		// End poking if MCP reaches an Updated state
		if updateComplete {
			t.Logf("%v: MCP %v is fully updated, exiting machine count check.", time.Now().Format(timeFmtStr), workerMcp.Name)
			break
		}

		// Wait `sleepInterval` seconds before checking MCP again
		if !(i == totalTime/sleepInterval-1) {
			time.Sleep(time.Duration(sleepInterval) * time.Second)
		}
	}

	// Delete the applied MC then throughout a 7 minute period or until the pool returns to an
	// updated state, check every 30 seconds that the updating and updated machine count populated
	// in the MCP matches the values calculated using historical functions.
	mcCleanupFunc()
	t.Log("Testing updated and degraded machine counts throughout MC deletion.")
	// for i := 0; i < totalTime/sleepInterval; i++ {
	// 	// Get `worker` MCP
	// 	workerMcp, mcpErr := cs.MachineConfigPools().Get(context.TODO(), workerMCPName, metav1.GetOptions{})
	// 	require.Nil(t, mcpErr, fmt.Sprintf("Error getting `%v` MCP: %v", workerMCPName, mcpErr))

	// 	// Get expected MCP machine counts from node annotations
	// 	workerNodes, nodeErr := helpers.GetNodesByRole(cs, workerMCPName)
	// 	require.Nil(t, nodeErr, fmt.Sprintf("Error getting nodes from `%v` MCP: %v", workerMCPName, nodeErr))
	// 	var workerNodesFormatted []*corev1.Node
	// 	for _, node := range workerNodes {
	// 		workerNodesFormatted = append(workerNodesFormatted, &node)
	// 	}
	// 	expectedUpdatedWorkerMachines := ctrlcommon.GetUpdatedMachines(workerMcp, workerNodesFormatted, &mcfgv1.MachineOSConfig{}, &mcfgv1.MachineOSBuild{}, false)
	// 	expectedDegradedWorkerMachines := ctrlcommon.GetDegradedMachines(workerNodesFormatted)

	// 	// Ensure the MCP updated and degraded machine counts match the expected counts
	// 	require.True(t, workerMcp.Status.UpdatedMachineCount == int32(len(expectedUpdatedWorkerMachines)), fmt.Sprintf("Updated machine count is %v, not the expected value of %v.", workerMcp.Status.UpdatedMachineCount, len(expectedUpdatedWorkerMachines)))
	// 	require.True(t, workerMcp.Status.DegradedMachineCount == int32(len(expectedDegradedWorkerMachines)), fmt.Sprintf("Degraded machine count is %v, not the expected value of %v", workerMcp.Status.DegradedMachineCount, len(expectedDegradedWorkerMachines)))

	// 	// End polling if MCP is returned to an Updated state

	// 	cond := false
	// 	for _, condition := range workerMcp.Status.Conditions {
	// 		if condition.Type == mcfgv1.MachineConfigPoolUpdated {
	// 			cond = true
	// 			break
	// 		}
	// 	}

	// 	if cond {
	// 		break
	// 	}

	// 	// Wait `sleepInterval` seconds before checking MCP again
	// 	time.Sleep(time.Duration(sleepInterval) * time.Second)
	// }

	// 	// Since this test case is for a valid MC, handle the error case when the MCP degrades
	// 	// TODO: handle degrade recovery
	// TODO: implement cleanup func
}

// func cleanupMC(t *testing.T, cs *framework.ClientSet, mcName string) {
// 	// Check if applied MC still exists
// 	mc, mcErr := cs.MachineConfigs().Get(context.TODO(), mcName, metav1.GetOptions{})
// 	if mcErr != nil {
// 		if
// 	}
// 	require.Nil(t, mcErr, "Error checking if MC `%v` exists.", mcName)

// 	// Delete applied MC
// 	t.Logf("Deleting MC '%v'.", mcName)
// 	deleteMCErr := oc.Run("delete").Args("machineconfig", mcName).Execute()
// 	o.Expect(deleteMCErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not delete MachineConfig '%v'.", mcName))

// 	// Wait for master MCP to be ready
// 	time.Sleep(15 * time.Second) //wait to not catch the updated state before the deleted mc triggers an update
// 	framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", poolName, 1)
// 	WaitForMCPToBeReady(oc, clientSet, poolName, 1)
// }
