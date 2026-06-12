package extended

import (
	"context"
	"path/filepath"
	"slices"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/machine-config-operator/test/extended-priv/util"
	logger "github.com/openshift/machine-config-operator/test/extended-priv/util/logext"
)

// [sig-mco][OCPFeatureGate:MachineConfigNodes] Should have MCN properties matching associated node properties for nodes in default MCPs [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/parallel] //GOOD
// [sig-mco][OCPFeatureGate:MachineConfigNodes] Should properly block MCN updates from a MCD that is not the associated one [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/parallel] //GOOD
// [sig-mco][OCPFeatureGate:MachineConfigNodes] Should properly block MCN updates by impersonation of the MCD SA [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/parallel] //GOOD
// [sig-mco][OCPFeatureGate:MachineConfigNodes] [Serial]Should have MCN properties matching associated node properties for nodes in custom MCPs [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/serial] //GOOD
// [sig-mco][OCPFeatureGate:MachineConfigNodes] [Serial]Should properly transition through MCN conditions on rebootless node update [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/serial] //GOOD
// [sig-mco][OCPFeatureGate:MachineConfigNodes] [Serial]Should properly update the MCN from the associated MCD [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/serial]
// [sig-mco][OCPFeatureGate:MachineConfigNodes] [Suite:openshift/machine-config-operator/disruptive][Disruptive]Should properly report MCN conditions on node degrade [apigroup:machineconfiguration.openshift.io] [Serial]
// [sig-mco][OCPFeatureGate:MachineConfigNodes] [Suite:openshift/machine-config-operator/disruptive][Disruptive][Slow]Should properly create and remove MCN on node creation and deletion [apigroup:machineconfiguration.openshift.io] [Serial]

// These tests verify MachineConfigNodes feature gate functionality.
var _ = g.Describe("[sig-mco][OCPFeatureGate:MachineConfigNodes]", func() {
	defer g.GinkgoRecover()
	var (
		// MCOMachineConfigPoolBaseDir    = exutil.FixturePath("testdata", "machine_config", "machineconfigpool")
		// MCOMachineConfigurationBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigurations")
		// MCOMachineConfigBaseDir        = exutil.FixturePath("testdata", "machine_config", "machineconfig")
		// infraMCPFixture                = filepath.Join(MCOMachineConfigPoolBaseDir, "infra-mcp.yaml")
		// nodeDisruptionFixture          = filepath.Join(MCOMachineConfigurationBaseDir, "nodedisruptionpolicy-rebootless-path.yaml")
		// nodeDisruptionEmptyFixture     = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-empty.yaml")
		customMCFixture = filepath.Join("machineconfigs", "infra-testfile-mc.yaml")
		masterMCFixture = filepath.Join("machineconfigs", "master-testfile-mc.yaml")
		// invalidWorkerMCFixture         = filepath.Join(MCOMachineConfigBaseDir, "1-worker-invalid-mc.yaml")
		// invalidMasterMCFixture         = filepath.Join(MCOMachineConfigBaseDir, "1-master-invalid-mc.yaml")
		oc = exutil.NewCLI("mco-image-mode-status", exutil.KubeConfigPath()).AsAdmin()
	)

	g.BeforeEach(func(ctx context.Context) {
		//skip these tests on hypershift platforms
		exutil.SkipOnHypershift(ctx, oc.AdminConfigClient())
	})

	// The following 3 tests are `Parallel` because they do not make any changes to the cluster and run quickly (< 5 min).
	g.It("Should have MCN properties matching associated node properties for nodes in default MCPs [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/parallel]", func() {
		ValidateMCNPropertiesByMCPs(oc)
	})

	g.It("Should properly block MCN updates from a MCD that is not the associated one [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/parallel]", func() {
		ValidateMCNScopeSadPathTest(oc)
	})

	g.It("Should properly block MCN updates by impersonation of the MCD SA [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/parallel]", func() {
		ValidateMCNScopeImpersonationPathTest(oc)
	})

	// The following 3 tests are `Serial` because they makes changes to the cluster that can impact other tests, but still run quickly (< 5 min).
	g.It("[Serial]Should have MCN properties matching associated node properties for nodes in custom MCPs [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/serial]", func() {
		// Get the MCPs in this cluster with machines. Since this cluster attempts to create a
		// custom MCP, the `worker` MCP must have machines for this test.
		clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")
		poolNames := GetRolesToTest(oc, clientSet)
		logger.Infof("Validating MCN properties for node(s) in pool(s) '%v'.", poolNames)
		if !slices.Contains(poolNames, "worker") {
			g.Skip("Skipping this test since this cluster has no machines in the worker MCP, so no custom MCP can be made.")
		}

		ValidateMCNPropertiesCustomMCP(oc, clientSet)
	})

	g.It("[Serial]Should properly transition through MCN conditions on rebootless node update [apigroup:machineconfiguration.openshift.io] [Suite:openshift/conformance/serial]", func() {
		// Skip this test when the `ImageModeStatusReporting` FeatureGate is enabled, since its
		// regression tests handle the different conditions list.
		exutil.SkipWhenFeatureGateEnabled(oc.AdminConfigClient(), "ImageModeStatusReporting")

		// Create client set for test
		clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

		// Get MCPs to test for cluster
		poolNames := GetRolesToTest(oc, clientSet)
		logger.Infof("Validating MCN properties for node(s) in pool(s) '%v'.", poolNames)

		// When the cluster has machines in the "worker" MCP, use a custom MCP to test the update
		if slices.Contains(poolNames, "worker") {
			logger.Infof("Validating MCN properties in custom MCP.")
			ValidateMCNConditionTransitionsOnRebootlessUpdate(oc, clientSet, nodeDisruptionFixture, nodeDisruptionEmptyFixture, customMCFixture)
		} else { // When there are no machines in the "worker" MCP, test the update by applying a MC targeting the "master" MCP
			logger.Infof("Validating MCN properties in master MCP.")
			ValidateMCNConditionTransitionsOnRebootlessUpdateMaster(oc, clientSet, nodeDisruptionFixture, nodeDisruptionEmptyFixture, masterMCFixture)
		}
	})

	// g.It("[Serial]Should properly update the MCN from the associated MCD [apigroup:machineconfiguration.openshift.io]", func() {
	// 	ValidateMCNScopeHappyPathTest(oc)
	// })

	// // This test is `Disruptive` because it degrades a node.
	// g.It("[Suite:openshift/machine-config-operator/disruptive][Disruptive]Should properly report MCN conditions on node degrade [apigroup:machineconfiguration.openshift.io]", func() {
	// 	if IsSingleNode(oc) { //handle SNO clusters
	// 		ValidateMCNConditionOnNodeDegrade(oc, invalidMasterMCFixture, true)
	// 	} else { //handle standard, non-SNO, clusters
	// 		ValidateMCNConditionOnNodeDegrade(oc, invalidWorkerMCFixture, false)
	// 	}
	// })

	// // This test is `Disruptive` because it creates and removes a node. It is also considered `Slow` because it takes longer than 5 min to run.
	// g.It("[Suite:openshift/machine-config-operator/disruptive][Disruptive][Slow]Should properly create and remove MCN on node creation and deletion [apigroup:machineconfiguration.openshift.io]", func() {
	// 	skipOnSingleNodeTopology(oc) //skip this test for SNO
	// 	ValidateMCNOnNodeCreationAndDeletion(oc)
	// })

})
