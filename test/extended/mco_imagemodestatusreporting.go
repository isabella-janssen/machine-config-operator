package extended

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/machine-config-operator/test/extended/util"
	logger "github.com/openshift/machine-config-operator/test/extended/util/logext"
)

var _ = g.Describe("[sig-mco][Suite:openshift/machine-config-operator/disruptive][Serial][Disruptive][OCPFeatureGate:ImageModeStatusReporting]", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("mco-imagemodestatusreporting", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		preChecks(oc)

		skipTestIfOCBIsEnabled(oc)
	})

	g.It("Should have correct machine count on custom MCP update for reboot-required node update", func() {
		// Since we are making a custom MCP in this test, it cannot be run on SNO or compact clusters.
		if IsCompactOrSNOCluster(oc.AsAdmin()) {
			g.Skip("The cluster is SNO/Compact. This test cannot be executed in SNO/Compact clusters.")
		}

		// Create custom MCP & add node to it
		// Apply MC
		// Check MCP machine counts throughout update
		// Delete MC
		// Check MCP machine counts throughout update
		// Remove custom MCP & check machine counts though process

		var (
			mcpName    = infraMcpName
			mcName     = fmt.Sprintf("90-%v-testfile", mcpName)
			fileConfig = getURLEncodedFileConfig("/etc/test-file", "hello", "420")
			workerMcp  = NewMachineConfigPool(oc.AsAdmin(), MachineConfigPoolWorker)
			nodeToTest = workerMcp.GetNodesOrFail()[0]
		)

		// Create custom `infra` MCP
		exutil.By("Create custom `infra` MCP")
		infraMcp, err := CreateCustomMCP(oc.AsAdmin(), infraMcpName, 0)
		defer infraMcp.delete()
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating a new custom MCP `%s`: %s", infraMcpName, err)
		logger.Infof("OK!\n")

		// Add a worker node to the new `infra` MCP
		exutil.By("Adding node to the `infra` MCP")
		o.Expect(
			nodeToTest.AddLabel(fmt.Sprintf("node-role.kubernetes.io/%s", infraMcpName), ""),
		).To(o.Succeed(),
			// TODO: update this to wait for the unlabeled node to return to pointing to the worker rendered config
			`Error moving the "%s" node to the "%s" pool`, nodeToTest, infraMcpName)
		defer nodeToTest.RemoveLabel(fmt.Sprintf("node-role.kubernetes.io/%s", infraMcpName))
		logger.Infof("OK!\n")

		// Create a MC to apply a file change
		exutil.By("Create a test file on node")
		mc := NewMachineConfig(oc.AsAdmin(), mcName, mcpName)
		mc.SetMCOTemplate(GenericMCTemplate)
		mc.SetParams(fmt.Sprintf("FILES=[%s]", fileConfig))
		mc.skipWaitForMcp = false
		defer mc.delete()
		mc.create()

		// exutil.By("Configure OCB functionality for the new infra MCP")
		// mosc, err := CreateMachineOSConfigUsingExternalOrInternalRegistry(oc.AsAdmin(), MachineConfigNamespace, moscName, infraMcpName, nil)
		// defer mosc.CleanupAndDelete()
		// o.Expect(err).NotTo(o.HaveOccurred(), "Error creating the MachineOSConfig resource")
		// logger.Infof("OK!\n")

		// ValidateSuccessfulMOSC(mosc, nil)

		// exutil.By("Remove the MachineOSConfig resource")
		// o.Expect(mosc.CleanupAndDelete()).To(o.Succeed(), "Error cleaning up %s", mosc)
		// logger.Infof("OK!\n")

		// ValidateMOSCIsGarbageCollected(mosc, infraMcp)

		// exutil.AssertAllPodsToBeReady(oc.AsAdmin(), MachineConfigNamespace)
		// logger.Infof("OK!\n")

	})
})

// // `ValidateMachineCountsInCustomMCPUpdate` validates that the machine counts populated in the
// // MCP status are correct throughout a standard update in a custom pool.
// func ValidateMachineCountsInCustomMCPUpdate(oc *exutil.CLI, nodeDisruptionFixture string, nodeDisruptionEmptyFixture string, mcFixture string, mcpFixture string) {
// 	mcpName := custom
// 	// mcName := fmt.Sprintf("90-%v-testfile", mcpName)

// 	// Create client set for test
// 	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
// 	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")
// 	kubeClient, kubeClientErr := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
// 	o.Expect(kubeClientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

// 	// Grab a random worker node
// 	nodeForTest := GetRandomNode(oc, worker)
// 	o.Expect(nodeForTest.Name).NotTo(o.Equal(""), "Could not get a worker node.")

// 	// Remove node disruption policy on test completion or failure
// 	defer func() {
// 		// Apply empty MachineConfiguration fixture to remove previously set NodeDisruptionPolicy
// 		framework.Logf("Removing node disruption policy.")
// 		ApplyMachineConfigurationFixture(oc, nodeDisruptionEmptyFixture)
// 	}()

// 	// Apply a node disruption policy to allow for rebootless update
// 	ApplyMachineConfigurationFixture(oc, nodeDisruptionFixture)

// 	// // Cleanup custom MCP, and delete MC on test completion or failure
// 	// defer func() {
// 	// 	cleanupErr := CleanupCustomMCP(oc, clientSet, custom, nodeForTest.Name, mcName)
// 	// 	o.Expect(cleanupErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed cleaning up '%v' MCP: %v.", custom, cleanupErr))
// 	// }()

// 	// Apply the fixture to create a custom MCP called "infra" & validate machine counts of new MCP
// 	mcpErr := oc.Run("apply").Args("-f", mcpFixture).Execute()
// 	o.Expect(mcpErr).NotTo(o.HaveOccurred(), "Could not create custom MCP.")
// 	// TODO: Add node count tracking in here
// 	// Observed behavior:
// 	// 	- MCP is added with no machines
// 	// 	- MCP updated, updating, degraded t/f values are updated later

// 	// Label the worker node accordingly to add it to the custom MCP
// 	labelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", nodeForTest.Name), fmt.Sprintf("node-role.kubernetes.io/%s=", custom)).Execute()
// 	o.Expect(labelErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not add label 'node-role.kubernetes.io/%s' to node '%v'.", custom, nodeForTest.Name))
// 	// TODO: Add node count tracking in here

// 	// Wait for the custom pool to be updated with the node ready
// 	framework.Logf("Waiting for '%v' MCP to be updated with %v ready machines.", mcpName, 1)
// 	WaitForMCPToBeReady(oc, clientSet, mcpName, 1)

// 	// Apply MC targeting custom MCP
// 	_, startingMCVersion := GetMCPConfigVersions(clientSet, mcpName)
// 	mcErr := oc.Run("apply").Args("-f", mcFixture).Execute()
// 	o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")
// 	// updatingNodeName := nodeForTest.Name
// 	// TODO: Add node count tracking in here
// 	framework.Logf("Validating machine counts on MC apply for `%v` MCP.", mcpName)
// 	ValidateMachineCountsOnMCPUpdate(clientSet, oc, kubeClient, mcpName, startingMCVersion, 10*time.Minute)

// 	// // Delete the applied MC
// 	// _, startingMCVersion = GetMCPConfigVersions(clientSet, mcpName)
// 	// mcErr = oc.Run("delete").Args("-f", mcFixture).Execute()
// 	// o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")
// 	// framework.Logf("Validating machine counts on MC delete for `%v` MCP.", mcpName)
// 	// ValidateMachineCountsOnMCPUpdate(clientSet, oc, kubeClient, workerMCPName, startingMCVersion, 5*time.Minute)

// 	// // Cleanup custom MCP
// 	// cleanupErr := CleanupCustomMCP(oc, clientSet, custom, nodeForTest.Name, mcName)
// 	// o.Expect(cleanupErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed cleaning up '%v' MCP: %v.", custom, cleanupErr))

// 	// // Check machine count on final "Updated"
// 	// framework.Logf("Checking all conditions other than 'Updated' are False.")
// 	// o.Expect(ConfirmUpdatedMCNStatus(clientSet, nodeForTest.Name)).Should(o.BeTrue(), "Error, all conditions must be 'False' when Updated=True.")
// }
