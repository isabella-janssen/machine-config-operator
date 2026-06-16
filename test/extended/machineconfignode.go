// TODO (MCO-1960): Deduplicate these functions with the helpers defined in /extended-priv/machineconfignode.go.
package extended

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	o "github.com/onsi/gomega"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	"github.com/openshift/machine-config-operator/pkg/daemon/constants"
	extpriv "github.com/openshift/machine-config-operator/test/extended-priv"
	exutil "github.com/openshift/machine-config-operator/test/extended-priv/util"
	logger "github.com/openshift/machine-config-operator/test/extended-priv/util/logext"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

// `ValidateMCNForNode` validates the MCN of a provided node by checking the following:
//   - Check that `mcn.Spec.Pool.Name` matches provided `poolName`
//   - Check that `mcn.Name` matches the node name
//   - Check that `mcn.Spec.ConfigVersion.Desired` matches the node desired config version
//   - Check that `nmcn.Status.ConfigVersion.Current` matches the node current config version
//   - Check that `mcn.Status.ConfigVersion.Desired` matches the node desired config version
//   - Check that `mcn.Spec.ConfigImage.DesiredImage` matches the node desired image
//   - Check that `nmcn.Status.ConfigImage.CurrentImage` matches the node current image
//   - Check that `mcn.Status.ConfigImage.DesiredImage` matches the node desired image
func ValidateMCNForNode(oc *exutil.CLI, machineConfigClient *machineconfigclient.Clientset, nodeName, poolName string) error {
	// Get updated node
	node, nodeErr := oc.AsAdmin().KubeClient().CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if nodeErr != nil {
		logger.Errorf("Could not get node `%v`", nodeName)
		return nodeErr
	}

	// Get node's desired and current config versions and images
	nodeCurrentConfig := node.Annotations[constants.CurrentMachineConfigAnnotationKey]
	nodeDesiredConfig := node.Annotations[constants.DesiredMachineConfigAnnotationKey]
	nodeCurrentImage := mcfgv1.ImageDigestFormat(node.Annotations[constants.CurrentImageAnnotationKey])
	nodeDesiredImage := mcfgv1.ImageDigestFormat(node.Annotations[constants.DesiredImageAnnotationKey])

	// Get node's MCN
	logger.Infof("Getting MCN for node `%v`.", nodeName)
	mcn, mcnErr := machineConfigClient.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if mcnErr != nil {
		logger.Errorf("Could not get MCN for node `%v`", nodeName)
		return mcnErr
	}

	// Check MCN pool name value for default MCPs
	logger.Infof("Checking MCN pool name for node `%v` matches pool association `%v`.", nodeName, poolName)
	if mcn.Spec.Pool.Name != poolName {
		logger.Errorf("MCN pool name `%v` does not match node MCP association `%v`.", mcn.Spec.Pool.Name, poolName)
		return fmt.Errorf("the MCN pool name does not match expected node MCP association")
	}

	// Check MCN name matches node name
	logger.Infof("Checking MCN name matches node name `%v`.", nodeName)
	if mcn.Name != nodeName {
		logger.Errorf("MCN name `%v` does not match node name `%v`.", mcn.Name, nodeName)
		return fmt.Errorf("the MCN name does not match node name")
	}

	// Check desired config version in MCN spec matches desired config on node
	logger.Infof("Checking node `%v` desired config version `%v` matches desired config version in MCN spec.", nodeName, nodeDesiredConfig)
	if mcn.Spec.ConfigVersion.Desired != nodeDesiredConfig {
		logger.Errorf("MCN spec desired config version `%v` does not match node desired config version `%v`.", mcn.Spec.ConfigVersion.Desired, nodeDesiredConfig)
		return fmt.Errorf("MCN spec desired config version does not match node desired config version")
	}

	// Check current config version in MCN status matches current config on node
	logger.Infof("Checking node `%v` current config version `%v` matches current version in MCN status.", nodeName, nodeCurrentConfig)
	if mcn.Status.ConfigVersion.Current != nodeCurrentConfig {
		logger.Infof("MCN status current config version `%v` does not match node current config version `%v`.", mcn.Status.ConfigVersion.Current, nodeCurrentConfig)
		return fmt.Errorf("MCN status current config version does not match node current config version")
	}

	// Check desired config version in MCN status matches desired config on node
	logger.Infof("Checking node `%v` desired config version `%v` matches desired version in MCN status.", nodeName, nodeDesiredConfig)
	if mcn.Status.ConfigVersion.Desired != nodeDesiredConfig {
		logger.Infof("MCN status desired config version `%v` does not match node desired config version `%v`.", mcn.Status.ConfigVersion.Desired, nodeDesiredConfig)
		return fmt.Errorf("MCN status desired config version does not match node desired config version")
	}

	// Check desired image in MCN spec matches desired image on node
	logger.Infof("Checking node `%v` desired image `%v` matches desired image in MCN spec.", nodeName, nodeDesiredImage)
	if mcn.Spec.ConfigImage.DesiredImage != nodeDesiredImage {
		logger.Errorf("MCN spec desired image `%v` does not match node desired image `%v`.", mcn.Spec.ConfigImage.DesiredImage, nodeDesiredImage)
		return fmt.Errorf("MCN spec desired image does not match node desired image")
	}

	// Check current image in MCN status matches current image on node
	logger.Infof("Checking node `%v` current image `%v` matches current image in MCN status.", nodeName, nodeCurrentImage)
	if mcn.Status.ConfigImage.CurrentImage != nodeCurrentImage {
		logger.Infof("MCN status current image `%v` does not match node current image `%v`.", mcn.Status.ConfigImage.CurrentImage, nodeCurrentImage)
		return fmt.Errorf("MCN status current image does not match node current image")
	}

	// Check desired image in MCN status matches desired image on node
	logger.Infof("Checking node `%v` desired image `%v` matches desired image in MCN status.", nodeName, nodeDesiredImage)
	if mcn.Status.ConfigImage.DesiredImage != nodeDesiredImage {
		logger.Infof("MCN status desired image `%v` does not match node desired image `%v`.", mcn.Status.ConfigImage.DesiredImage, nodeDesiredImage)
		return fmt.Errorf("MCN status desired image does not match node desired image")
	}

	return nil
}

// `getMCNCondition` returns the queried condition or nil if the condition does not exist
func getMCNCondition(mcn *mcfgv1.MachineConfigNode, conditionType mcfgv1.StateProgress) *metav1.Condition {
	// Loop through conditions and return the status of the desired condition type
	conditions := mcn.Status.Conditions
	for _, condition := range conditions {
		if condition.Type == string(conditionType) {
			return &condition
		}
	}
	return nil
}

// `getMCNConditionStatus` returns the status of the desired condition type for MCN, or
// an empty string if the condition does not exist
func getMCNConditionStatus(mcn *mcfgv1.MachineConfigNode, conditionType mcfgv1.StateProgress) metav1.ConditionStatus {
	// Loop through conditions and return the status of the desired condition type
	condition := getMCNCondition(mcn, conditionType)
	if condition == nil {
		return ""
	}

	logger.Infof("MCN '%s' %s condition status is %s", mcn.Name, conditionType, condition.Status)
	return condition.Status
}

// `checkMCNConditionStatus` checks that an MCN condition matches the desired status (ex.
// confirm "Updated" is "False")
func checkMCNConditionStatus(mcn *mcfgv1.MachineConfigNode, conditionType mcfgv1.StateProgress, status metav1.ConditionStatus) bool {
	conditionStatus := getMCNConditionStatus(mcn, conditionType)
	if conditionStatus != status && conditionType == mcfgv1.MachineConfigNodeResumed {
		condition := getMCNCondition(mcn, conditionType)
		logger.Infof("LastTransitionTime: %v, Message: %v, ObservedGeneration: %v, Reason: %v, Status: %v, Type: %v", condition.LastTransitionTime, condition.Message, condition.ObservedGeneration, condition.Reason, condition.Status, condition.Type)
	}
	return conditionStatus == status
}

// `waitForMCNConditionStatus` waits up to a specified timeout for the desired MCN condition to
// match the desired status (ex. wait until "Updated" is "False"). If the desired condition is
// "Unknown," the function will also return true if the condition is "True," which ensures that we
// do not fail when an update progresses quickly through the intermediary "Unknown" phase.
func waitForMCNConditionStatus(machineConfigClient *machineconfigclient.Clientset, mcnName string, conditionType mcfgv1.StateProgress, status metav1.ConditionStatus,
	timeout time.Duration, interval time.Duration) (bool, error) {

	conditionMet := false
	var conditionErr error
	var workerNodeMCN *mcfgv1.MachineConfigNode
	if err := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(_ context.Context) (bool, error) {
		logger.Infof("Waiting for MCN '%v' %v condition to be %v.", mcnName, conditionType, status)

		workerNodeMCN, conditionErr = machineConfigClient.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), mcnName, metav1.GetOptions{})
		// Record if an error occurs when getting the MCN resource
		if conditionErr != nil {
			logger.Infof("Error getting MCN for node '%v': %v", mcnName, conditionErr)
			return false, nil
		}

		// Check if the MCN status is as desired
		conditionMet = checkMCNConditionStatus(workerNodeMCN, conditionType, status)
		// If the condition was not met and we are expecting it may have transitioned quickly
		// trough the "Unknown" phase, check if the condition has flipped to `True`.
		if !conditionMet && status == metav1.ConditionUnknown {
			conditionMet = checkMCNConditionStatus(workerNodeMCN, conditionType, metav1.ConditionTrue)
			logger.Infof("MCN '%v' %v condition was %v, missed transition through %v.", mcnName, conditionType, metav1.ConditionTrue, status)
		}
		return conditionMet, nil
	}); err != nil {
		logger.Infof("The desired MCN condition was never met: %v", err)
		// Handle the situation where there were errors getting the MCN resource
		if conditionErr != nil {
			logger.Infof("An error occurred waiting for MCN '%v' %v condition to be %v: %v", mcnName, conditionType, status, conditionErr)
			return conditionMet, fmt.Errorf("MCN '%v' %v condition was not %v: %v", mcnName, conditionType, status, conditionErr)
		}
		// Handle case when no errors occur grabbing the MCN, but we time out waiting for the condition to be in the desired state
		logger.Infof("A timeout occurred waiting for MCN '%v' %v condition was not %v.", mcnName, conditionType, status)
		return conditionMet, nil
	}

	return conditionMet, conditionErr
}

// `ValidateTransitionThroughConditions` validates the condition trasnitions in the MCN
// during a node update. Note that some conditions are passed through quickly in a node update, so
// the test can "miss" catching the phases. For test stability, if we fail to catch an "Unknown"
// status, a warning will be logged instead of erroring out the test.
//
//nolint:dupl // (ijanssen): Ignoring a duplication error the linter is throwing because of two similar, but unique if blocks.
func ValidateTransitionThroughConditions(machineConfigClient *machineconfigclient.Clientset, updatingNodeName string, isRebootless, isImageMode bool) {
	// Get the start time of the update
	updateStartTime := metav1.Now()

	// Set the expected wait time for the MCN condition of a working node to flip to Updated=False.
	// For image-based updates, this condition flip can take a while since an image needs to build
	// and push before the nodes uppdate, but we want to make sure non-image updates do not take as
	// long for the condition to flip (that would mean something is wrong and would waste time).
	updatingWaitTime := 1 * time.Minute
	updatingWaitInterval := 1 * time.Second
	if isImageMode {
		updatingWaitTime = 25 * time.Minute
		updatingWaitInterval = 5 * time.Second
	}

	// Test the condition transitions.
	logger.Infof("Waiting for Updated=False")
	conditionMet, err := waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdated, metav1.ConditionFalse, updatingWaitTime, updatingWaitInterval)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for Updated=False: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Updated=False.")

	logger.Infof("Waiting for UpdatePrepared=True")
	conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdatePrepared, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for UpdatePrepared=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdatePrepared=True.")

	logger.Infof("Waiting for UpdateExecuted=Unknown")
	conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateExecuted, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for UpdateExecuted=Unknown: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateExecuted=Unknown.")

	// On standard, non-rebootless, update, check that node transitions through "Cordoned" and "Drained" phases
	if !isRebootless {
		logger.Infof("Waiting for Cordoned=True")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateCordoned, metav1.ConditionTrue, 30*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for Cordoned=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Cordoned=True.")

		logger.Infof("Waiting for Drained=Unknown")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateDrained, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for Drained=Unknown: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Drained=Unknown.")

		logger.Infof("Waiting for Drained=True")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateDrained, metav1.ConditionTrue, 4*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for Drained=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Drained=True.")
	}

	// On image mode update, check that node transitions through the "AppliedOSImage" and
	// "ImagePulledFromRegistry" phases
	if isImageMode {
		logger.Infof("Waiting for AppliedOSImage=Unknown")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateOS, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for AppliedOSImage=Unknown: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect AppliedOSImage=Unknown.")

		logger.Infof("Waiting for ImagePulledFromRegistry=Unknown")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeImagePulledFromRegistry, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for ImagePulledFromRegistry=Unknown: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect ImagePulledFromRegistry=Unknown.")

		logger.Infof("Waiting for AppliedOSImage=True")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateOS, metav1.ConditionTrue, 3*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for AppliedOSImage=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect AppliedOSImage=True.")
	} else { // On a non-image mode update, check that node transitions through the "AppliedFiles" phase
		logger.Infof("Waiting for AppliedFiles=Unknown")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateFiles, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for AppliedFiles=Unknown: %v", err))
		if !conditionMet {
			logger.Infof("Warning, could not detect AppliedFiles=Unknown.")
		}

		logger.Infof("Waiting for AppliedFiles=True")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateFiles, metav1.ConditionTrue, 3*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for AppliedFiles=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect AppliedFiles=True.")
	}

	logger.Infof("Waiting for UpdateExecuted=True")
	conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateExecuted, metav1.ConditionTrue, 20*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for UpdateExecuted=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateExecuted=True.")

	// On image mode update, check that node transitions through the "ImagePulledFromRegistry" phase
	if isImageMode {
		logger.Infof("Waiting for ImagePulledFromRegistry=True")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeImagePulledFromRegistry, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for ImagePulledFromRegistry=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect ImagePulledFromRegistry=True.")
	}

	// On rebootless update, check that node transitions through "UpdatePostActionComplete" phase
	if isRebootless {
		logger.Infof("Waiting for UpdatePostActionComplete=True")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdatePostActionComplete, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for UpdatePostActionComplete=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdatePostActionComplete=True.")
	} else { // On standard, non-rebootless, update, check that node transitions through "RebootedNode" phase
		logger.Infof("Waiting for RebootedNode=Unknown")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateRebooted, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for RebootedNode=Unknown: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect RebootedNode=Unknown.")

		logger.Infof("Waiting for RebootedNode=True")
		conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateRebooted, metav1.ConditionTrue, 10*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for RebootedNode=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect RebootedNode=True.")
	}

	// The final steps of the update happen quickly, so sometimes we can miss the final condition
	// transitions. If we do, we will not error out, but record that the condition was missed.
	logger.Infof("Waiting for Resumed=True")
	conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeResumed, metav1.ConditionTrue, 5*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for Resumed=True: %v", err))
	if !conditionMet {
		logger.Infof("Warning, could not detect Resumed=True.")
	}
	logger.Infof("Waiting for UpdateComplete=True")
	conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateComplete, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for UpdateComplete=True: %v", err))
	if !conditionMet {
		logger.Infof("Warning, could not detect UpdateComplete=True.")
	}
	logger.Infof("Waiting for Uncordoned=True")
	conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdateUncordoned, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for UpdateComplete=True: %v", err))
	if !conditionMet {
		logger.Infof("Warning, could not detect UpdateComplete=True.")
	}

	logger.Infof("Waiting for Updated=True")
	conditionMet, err = waitForMCNConditionStatus(machineConfigClient, updatingNodeName, mcfgv1.MachineConfigNodeUpdated, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while waiting for Updated=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Updated=True.")

	// When the update is not an image mode update, we need to check that the
	// "ImagePulledFromRegistry" condition did not transition during the updates
	if !isImageMode {
		mcn, mcnErr := machineConfigClient.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), updatingNodeName, metav1.GetOptions{})
		o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occurred while trying to get MCN for node `%v`: %v", updatingNodeName, mcnErr))

		for _, condition := range mcn.Status.Conditions {
			if condition.Type == string(mcfgv1.MachineConfigNodeImagePulledFromRegistry) {
				logger.Infof("Checking that %v was not updated during this update.", condition.Type)
				o.Expect(condition.LastTransitionTime.Before(&updateStartTime)).To(o.BeTrue(), "Expected last transition time of %v condition to be before %v.", condition.Type, updateStartTime)
			}
		}
	}
}

// `ConfirmUpdatedMCNStatus` confirms that an MCN is in a fully updated state, which requires:
//  1. "Updated" = True
//  2. All other conditions = False
func ConfirmUpdatedMCNStatus(clientSet *machineconfigclient.Clientset, mcnName string) bool {
	// Get MCN
	workerNodeMCN, workerErr := clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), mcnName, metav1.GetOptions{})
	o.Expect(workerErr).NotTo(o.HaveOccurred())

	// Loop through conditions and return the status of the desired condition type
	conditions := workerNodeMCN.Status.Conditions
	for _, condition := range conditions {
		if condition.Type == string(mcfgv1.MachineConfigNodeUpdated) && condition.Status != metav1.ConditionTrue {
			logger.Infof("Node '%s' update is not complete; 'Updated' condition status is '%v'", mcnName, condition.Status)
			return false
		} else if condition.Type != string(mcfgv1.MachineConfigNodeUpdated) && condition.Status != metav1.ConditionFalse {
			logger.Infof("Node '%s' is updated but MCN is invalid; '%v' codition status is '%v'", mcnName, condition.Type, condition.Status)
			return false
		}
	}

	logger.Infof("Node '%s' update is complete and corresponding MCN is valid.", mcnName)
	return true
}

// `ValidateMCNPropertiesByMCPs` checks that MCN properties match the corresponding node properties
// for a random node in each MCP in the cluster with nodes.
func ValidateMCNPropertiesByMCPs(oc *exutil.CLI) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Get MCPs to test for cluster
	poolNames := GetRolesToTest(oc, clientSet)
	logger.Infof("Validating MCN properties for node(s) in pool(s) '%v'.", poolNames)

	// Validate MCN associated with node in each desired MCP
	for _, poolName := range poolNames {
		logger.Infof("Validating MCN properties for %v node.", poolName)

		// Grab a node in the desired MCP
		node := GetRandomNode(oc, poolName)
		o.Expect(node.Name).NotTo(o.Equal(""), fmt.Sprintf("Could not get a %v node.", poolName))

		// Validate MCN for the MCP's node
		logger.Infof("Validating MCN properties for the node '%v'.", node.Name)
		mcnErr := ValidateMCNForNode(oc, clientSet, node.Name, poolName)
		o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties for the node in pool '%v'.", poolName))
	}
}

// `ValidateMCNScopeSadPathTest` checks that MCN updates from a MCD that is not the associated one are
// blocked. This test skips on SNO clusters.
func ValidateMCNScopeSadPathTest(oc *exutil.CLI) {
	// Get all nodes from the cluster
	nodes, nodesErr := GetAllNodes(oc)
	o.Expect(nodesErr).NotTo(o.HaveOccurred(), "Error getting nodes from cluster: %v", nodesErr)
	o.Expect(len(nodes)).To(o.BeNumerically(">", 0), "Got 0 nodes from cluster.")

	// If cluster is SNO (has only one node), skip this test
	if len(nodes) == 1 {
		e2eskipper.Skipf("This test does not apply to single-node topologies")
	}

	// Grab two different nodes, so we don't end up testing and targeting the same node.
	nodeUnderTest := nodes[0]
	targetNode := nodes[1]
	logger.Infof("Testing with nodes '%v' and '%v'.", nodeUnderTest.Name, targetNode.Name)

	// Attempt to patch the MCN owned by targetNode from nodeUnderTest's MCD. This should fail.
	// This oc command effectively use the service account of the nodeUnderTest's MCD pod, which should only be able to edit nodeUnderTest's MCN.
	cmdOutput, err := ExecCmdOnNodeWithError(oc, nodeUnderTest, "chroot", "/rootfs", "oc", "patch", "machineconfignodes", targetNode.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}")
	o.Expect(err).To(o.HaveOccurred())
	logger.Infof("MCN patch was successfully blocked.")
	o.Expect(cmdOutput).To(o.ContainSubstring("updates to MCN " + targetNode.Name + " can only be done from the MCN's owner node"))
	logger.Infof("Error string contains desired substring.")
}

// `ValidateMCNScopeImpersonationPathTest` checks that MCN updates by impersonation of the MCD SA are blocked
func ValidateMCNScopeImpersonationPathTest(oc *exutil.CLI) {
	// Grab a random node with a worker role
	nodeUnderTest := GetRandomNode(oc, "worker")
	o.Expect(nodeUnderTest.Name).NotTo(o.Equal(""), "Could not get a `worker` node.")
	logger.Infof("Testing with node '%v'.", nodeUnderTest.Name)

	var errb bytes.Buffer
	// Attempt to patch the MCN owned by nodeUnderTest by impersonating the MCD SA. This should fail.
	cmd := exec.Command("oc", "patch", "machineconfignodes", nodeUnderTest.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}", "--as=system:serviceaccount:openshift-machine-config-operator:machine-config-daemon")
	cmd.Stderr = &errb
	err := cmd.Run()

	o.Expect(err).To(o.HaveOccurred())
	logger.Infof("MCN patch was successfully blocked.")
	o.Expect(errb.String()).To(o.ContainSubstring("this user must have a \"authentication.kubernetes.io/node-name\" claim"))
	logger.Infof("Error string contains desired substring.")
}

// `ValidateMCNScopeHappyPathTest` checks that MCN updates from the associated MCD are allowed
func ValidateMCNScopeHappyPathTest(oc *exutil.CLI) {
	// Grab a random node with a worker role
	nodeUnderTest := GetRandomNode(oc, "worker")
	o.Expect(nodeUnderTest.Name).NotTo(o.Equal(""), "Could not get a `worker` node.")
	logger.Infof("Testing with node '%v'.", nodeUnderTest.Name)

	// Get node's starting desired version
	nodeDesiredConfig := nodeUnderTest.Annotations["machineconfiguration.openshift.io/desiredConfig"]

	// Attempt to patch the MCN owned by nodeUnderTest from nodeUnderTest's MCD. This should succeed.
	// This oc command effectively use the service account of the nodeUnderTest's MCD pod, which should only be able to edit nodeUnderTest's MCN.
	ExecCmdOnNode(oc, nodeUnderTest, "chroot", "/rootfs", "oc", "patch", "machineconfignodes", nodeUnderTest.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}")
	logger.Infof("MCN '%v' patched successfully.", nodeUnderTest.Name)

	// Cleanup by updating the MCN desired config back to the original value.
	logger.Infof("Cleaning up patched MCN's desired config value.")
	ExecCmdOnNode(oc, nodeUnderTest, "chroot", "/rootfs", "oc", "patch", "machineconfignodes", nodeUnderTest.Name, "--type=merge", "-p", fmt.Sprintf("{\"spec\":{\"configVersion\":{\"desired\":\"%v\"}}}", nodeDesiredConfig))
	logger.Infof("MCN successfully cleaned up.")
}

// `ValidateMCNPropertiesCustomMCP` checks that MCN properties match the corresponding node properties
func ValidateMCNPropertiesCustomMCP(oc *exutil.CLI, clientSet *machineconfigclient.Clientset) {
	// Grab a random node from each default pool
	workerNode := GetRandomNode(oc, "worker")
	o.Expect(workerNode.Name).NotTo(o.Equal(""), "Could not get a worker node.")

	// Create MCP
	_, err := extpriv.CreateCustomMCP(oc.AsAdmin(), "infra", 0)
	defer CleanupCustomMCP(oc, clientSet, "infra", workerNode.Name)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error creating a new custom pool `%s`: %s", "infra", err)
	// Label node
	err = oc.AsAdmin().Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s=", "infra")).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "Error labeing node `%s` for MCP `%s`: %s", workerNode.Name, "infra", err)
	// Wait for the new `infra` MCP to be ready
	WaitForMCPToBeReady(clientSet, "infra", 1, "")
	logger.Infof("OK!\n")

	// Get node in custom pool
	customNodes, customNodeErr := GetNodesByRole(oc, "infra")
	o.Expect(customNodeErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not get node in MCP '%v'.", "infra"))
	customNode := customNodes[0]

	// Validate MCN for node in custom pool
	logger.Infof("Validating MCN properties for node in custom '%v' pool.", "infra")
	mcnErr := ValidateMCNForNode(oc, clientSet, customNode.Name, "infra")
	o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties node in custom pool '%v'.", "infra"))
}

// `validateTransitionThroughConditions` validates the condition trasnitions in the MCN during a node update
func validateTransitionThroughConditions(clientSet *machineconfigclient.Clientset, updatingNodeName string, isRebootless bool) {
	// Note that some conditions are passed through quickly in a node update, so the test can
	// "miss" catching the phases. For test stability, if we fail to catch an "Unknown" status,
	// a warning will be logged instead of erroring out the test.
	logger.Infof("Waiting for Updated=False")
	conditionMet, err := waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdated, metav1.ConditionFalse, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Updated=False: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Updated=False.")

	logger.Infof("Waiting for UpdatePrepared=True")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdatePrepared, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdatePrepared=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdatePrepared=True.")

	logger.Infof("Waiting for UpdateExecuted=Unknown")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateExecuted, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdateExecuted=Unknown: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateExecuted=Unknown.")

	// On standard, non-rebootless, update, check that node transitions through "Cordoned" and "Drained" phases
	if !isRebootless {
		logger.Infof("Waiting for Cordoned=True")
		conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateCordoned, metav1.ConditionTrue, 30*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Cordoned=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Cordoned=True.")

		logger.Infof("Waiting for Drained=Unknown")
		conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateDrained, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Drained=Unknown: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Drained=Unknown.")

		logger.Infof("Waiting for Drained=True")
		conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateDrained, metav1.ConditionTrue, 4*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Drained=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Drained=True.")
	}

	logger.Infof("Waiting for AppliedFilesAndOS=Unknown")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for AppliedFilesAndOS=Unknown: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect AppliedFilesAndOS=Unknown.")

	logger.Infof("Waiting for AppliedFilesAndOS=True")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionTrue, 3*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for AppliedFilesAndOS=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect AppliedFilesAndOS=True.")

	logger.Infof("Waiting for UpdateExecuted=True")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateExecuted, metav1.ConditionTrue, 20*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdateExecuted=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateExecuted=True.")

	// On rebootless update, check that node transitions through "UpdatePostActionComplete" phase
	if isRebootless {
		logger.Infof("Waiting for UpdatePostActionComplete=True")
		conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdatePostActionComplete, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdatePostActionComplete=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdatePostActionComplete=True.")
	} else { // On standard, non-rebootless, update, check that node transitions through "RebootedNode" phase
		logger.Infof("Waiting for RebootedNode=Unknown")
		conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateRebooted, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for RebootedNode=Unknown: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect RebootedNode=Unknown.")

		logger.Infof("Waiting for RebootedNode=True")
		conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateRebooted, metav1.ConditionTrue, 6*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for RebootedNode=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect RebootedNode=True.")
	}
	logger.Infof("Waiting for Resumed=True")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeResumed, metav1.ConditionTrue, 15*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Resumed=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Resumed=True.")

	logger.Infof("Waiting for UpdateComplete=True")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateComplete, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdateComplete=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateComplete=True.")

	logger.Infof("Waiting for Uncordoned=True")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateUncordoned, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdateComplete=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateComplete=True.")

	logger.Infof("Waiting for Updated=True")
	conditionMet, err = waitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdated, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Updated=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Updated=True.")
}

// `ValidateMCNConditionTransitionsOnRebootlessUpdate` checks that the `Conditions` in an MCN
// properly update on a node update in a custom MCP. The steps of this function are:
//  1. Apply a node disruption policy
//  2. Create a custom MCP with one node
//  3. Apply a MC
//  4. Validate the MCN conditions transition as expected throughout the update
//  5. Clean up the test resources
func ValidateMCNConditionTransitionsOnRebootlessUpdate(oc *exutil.CLI, clientSet *machineconfigclient.Clientset, nodeDisruptionFixture, nodeDisruptionEmptyFixture, mcFixture string) {
	poolName := "infra"
	mcName := fmt.Sprintf("90-%v-testfile", poolName)

	// Grab a random worker node
	exutil.By("Getting node to test")
	workerNode := GetRandomNode(oc, "worker")
	o.Expect(workerNode.Name).NotTo(o.Equal(""), "Could not get a worker node.")
	logger.Infof("OK!\n")

	// Apply a node disruption policy to allow for a rebootless update
	exutil.By("Applying the NodeDisruptionPolicy")
	err := ApplyMachineConfigFixture(oc, nodeDisruptionFixture)
	defer ApplyMachineConfigFixture(oc, nodeDisruptionEmptyFixture)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error applying the NodeDisruptionPolicy: %s", err)
	logger.Infof("OK!\n")

	exutil.By("Create custom `infra` MCP and add the test node to it")
	// Create MCP
	_, err = extpriv.CreateCustomMCP(oc.AsAdmin(), poolName, 0)
	defer CleanupCustomMCP(oc, clientSet, poolName, workerNode.Name)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error creating a new custom pool `%s`: %s", poolName, err)
	// Label node
	err = oc.AsAdmin().Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s=", poolName)).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "Error labeing node `%s` for MCP `%s`: %s", workerNode.Name, poolName, err)
	// Wait for the new `infra` MCP to be ready
	WaitForMCPToBeReady(clientSet, poolName, 1, "")
	logger.Infof("OK!\n")

	// Apply MC targeting custom pool node
	exutil.By("Applying the MC")
	mcErr := oc.Run("apply").Args("-f", mcFixture).Execute()
	defer DeleteMCAndWaitForMCPUpdate(oc, clientSet, mcName, poolName)
	o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")
	updatingNodeName := workerNode.Name
	logger.Infof("OK!\n")

	// Validate transition through conditions for MCN
	validateTransitionThroughConditions(clientSet, updatingNodeName, true)

	// When an update is complete, all conditions other than `Updated` must be false
	logger.Infof("Checking all conditions other than 'Updated' are False.")
	o.Expect(ConfirmUpdatedMCNStatus(clientSet, updatingNodeName)).Should(o.BeTrue(), "Error, all conditions must be 'False' when Updated=True.")
}

// `ValidateMCNConditionTransitionsOnRebootlessUpdateMaster` checks that the `Conditions` in an MCN
// properly update on a node update in the master MCP. The steps of this function are:
//  1. Apply a node disruption policy
//  2. Apply a MC
//  3. Get the updating node
//  4. Validate the MCN conditions transition as expected throughout the update
//  5. Clean up the test resources
func ValidateMCNConditionTransitionsOnRebootlessUpdateMaster(oc *exutil.CLI, clientSet *machineconfigclient.Clientset, nodeDisruptionFixture string, nodeDisruptionEmptyFixture string, mcFixture string) {
	poolName := "master"
	mcName := fmt.Sprintf("90-%v-testfile", poolName)

	// Get the starting config version & machine count
	mcp, mcpErr := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	o.Expect(mcpErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not get MCP '%v'; %v", poolName, mcpErr))
	startingConfigVersion := mcp.Spec.Configuration.Name
	// machineCount := mcp.Status.MachineCount

	// Apply a node disruption policy to allow for a rebootless update
	exutil.By("Applying the NodeDisruptionPolicy")
	err := ApplyMachineConfigFixture(oc, nodeDisruptionFixture)
	defer ApplyMachineConfigFixture(oc, nodeDisruptionEmptyFixture)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error applying the NodeDisruptionPolicy: %s", err)
	logger.Infof("OK!\n")

	exutil.By("Applying the MC")
	err = ApplyMachineConfigFixture(oc, mcNameToFixtureMap[mcName])
	defer DeleteMCAndWaitForMCPUpdate(oc, clientSet, mcName, poolName)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error applying MC `%s`: %s", mcName, err)
	logger.Infof("OK!\n")

	// Apply MC targeting master MCP
	mcErr := oc.Run("apply").Args("-f", mcFixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")

	// Get the updating node
	updatingNode := GetUpdatingNode(oc, poolName, startingConfigVersion)
	o.Expect(updatingNode).NotTo(o.BeNil(), "Could not get updating node.")
	logger.Infof("Node '%v' is updating.", updatingNode.Name)

	// Validate transition through conditions for MCN
	validateTransitionThroughConditions(clientSet, updatingNode.Name, true)

	// When an update is complete, all conditions other than `Updated` must be false
	logger.Infof("Checking all conditions other than 'Updated' are False.")
	o.Expect(ConfirmUpdatedMCNStatus(clientSet, updatingNode.Name)).Should(o.BeTrue(), "Error, all conditions must be 'False' when Updated=True.")
}

// `ValidateMCNConditionOnNodeDegrade` checks that Conditions properly update on a node failure (MCP degrade)
func ValidateMCNConditionOnNodeDegrade(oc *exutil.CLI, fixture string, isSno bool) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// In SNO, master pool will degrade
	poolName := "worker"
	mcName := "91-worker-testfile-invalid"
	if isSno {
		poolName = "master"
		mcName = "91-master-testfile-invalid"
	}

	var degradedNodeMCN *mcfgv1.MachineConfigNode
	// Cleanup MC and fix node degradation on failure or test completion
	defer func() {
		// Delete the applied MC
		deleteMCErr := oc.Run("delete").Args("machineconfig", mcName).Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not delete MachineConfig '%v'.", mcName))

		// Recover the degraded MCP
		recoverErr := RecoverFromDegraded(oc, poolName)
		o.Expect(recoverErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not recover MCP '%v' from degraded state.", poolName))

		// If the test reached checking the MCN ensure the NodeDegraded condition is properly restored
		if degradedNodeMCN != nil {
			conditionMet, err := waitForMCNConditionStatus(clientSet, degradedNodeMCN.Name, mcfgv1.MachineConfigNodeNodeDegraded, metav1.ConditionFalse, 30*time.Second, 1*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for NodeDegraded=False: %v", err))
			o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect NodeDegraded=False.")
		}
	}()

	// Apply invalid MC
	mcErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")

	// Wait for MCP to be in a degraded state with one degraded machine
	degradedErr := WaitForMCPConditionStatus(oc, poolName, "Degraded", corev1.ConditionTrue, 8*time.Minute, 3*time.Second)
	o.Expect(degradedErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error waiting for '%v' MCP to be in a degraded state.", poolName))
	mcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting '%v' MCP.", poolName))
	o.Expect(mcp.Status.DegradedMachineCount).To(o.BeNumerically("==", 1), fmt.Sprintf("Degraded machine count is not 1. It is %v.", mcp.Status.DegradedMachineCount))

	// Get degraded node
	degradedNode, degradedNodeErr := GetDegradedNode(oc, poolName)
	o.Expect(degradedNodeErr).NotTo(o.HaveOccurred(), "Could not get degraded node.")

	unknownCondition := mcfgv1.MachineConfigNodeUpdateFilesAndOS
	if exutil.IsFeatureGateEnabled(oc.AdminConfigClient(), "ImageModeStatusReporting") {
		unknownCondition = mcfgv1.MachineConfigNodeUpdateFiles
	}

	// Validate MCN of degraded node
	// 	get and log MCN conditions for debugging purposes
	degradedNodeMCN, degradedErr = clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), degradedNode.Name, metav1.GetOptions{})
	o.Expect(degradedErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting MCN of degraded node '%v'.", degradedNode.Name))
	nodeDegradedCondition := getMCNCondition(degradedNodeMCN, mcfgv1.MachineConfigNodeNodeDegraded)
	o.Expect(nodeDegradedCondition).NotTo(o.BeNil(), "Condition 'NodeDegraded' does not exist.")
	logger.Infof("`NodeDegraded` condition status is `%v` with the message `%v`", nodeDegradedCondition.Status, nodeDegradedCondition.Message)
	fileCondition := getMCNCondition(degradedNodeMCN, unknownCondition)
	o.Expect(fileCondition).NotTo(o.BeNil(), "Condition '%v' does not exist.", unknownCondition)
	logger.Infof("`%v` condition status is `%v` with the message `%v`", unknownCondition, fileCondition.Status, fileCondition.Message)
	executedCondition := getMCNCondition(degradedNodeMCN, mcfgv1.MachineConfigNodeUpdateExecuted)
	o.Expect(executedCondition).NotTo(o.BeNil(), "Condition 'UpdateExecuted' does not exist.")
	logger.Infof("`UpdateExecuted` condition status is `%v` with the message `%v`", executedCondition.Status, executedCondition.Message)
	// 	validate the conditions are as expected
	logger.Infof("Validating that `NodeDegraded` condition in '%v' MCN has a status of 'True'.", degradedNodeMCN.Name)
	o.Expect(nodeDegradedCondition.Status).Should(o.Equal(metav1.ConditionTrue), "Condition 'NodeDegraded' does not have the expected status of 'True'.")
	o.Expect(nodeDegradedCondition.Message).Should(o.ContainSubstring(fmt.Sprintf("Node %s upgrade failure.", degradedNodeMCN.Name)), "Condition 'NodeDegraded' does not have the expected message.")
	o.Expect(nodeDegradedCondition.Message).Should(o.ContainSubstring("/home/core: file exists"), "Condition 'NodeDegraded' does not have the expected message details.")
	logger.Infof("Validating that `UpdateExecuted` condition in '%v' MCN has a status of 'Unknown'.", degradedNodeMCN.Name)
	o.Expect(executedCondition.Status).Should(o.Equal(metav1.ConditionUnknown), "Condition 'UpdateExecuted' does not have the expected status of 'Unknown'.")
	logger.Infof("Validating that `%v` condition in '%v' MCN has a status of 'Unknown'.", unknownCondition, degradedNodeMCN.Name)
	o.Expect(fileCondition.Status).Should(o.Equal(metav1.ConditionUnknown), "Condition '%v' does not have the expected status of 'Unknown'.", unknownCondition)
}

// `RecoverFromDegraded` gets the degraded node in the desired MCP, forces the node to recover by updating its desired
// config to be its current config, and waits for the MCP to return to an Update=True state
func RecoverFromDegraded(oc *exutil.CLI, mcpName string) error {
	logger.Infof("Recovering %s pool from degraded state", mcpName)

	// Get nodes from degraded MCP & update the desired config of the degraded node to force a recovery update
	nodes, nodeErr := GetNodesByRole(oc, mcpName)
	o.Expect(nodeErr).NotTo(o.HaveOccurred())
	o.Expect(nodes).ShouldNot(o.BeEmpty())
	for _, node := range nodes {
		logger.Infof("Restoring desired config for node: %s", node.Name)
		if checkMCDState(node, "Done") {
			logger.Infof("Node %s is updated and does not need to be recovered", node.Name)
		} else {
			err := restoreDesiredConfig(oc, node)
			if err != nil {
				return fmt.Errorf("error restoring desired config in node %s. Error: %s", node.Name, err)
			}
		}
	}

	// Wait for MCP to not be in degraded status
	mcpErr := WaitForMCPConditionStatus(oc, mcpName, "Degraded", "False", 4*time.Minute, 5*time.Second)
	o.Expect(mcpErr).NotTo(o.HaveOccurred(), fmt.Sprintf("could not recover %v MCP from the degraded status.", mcpName))
	mcpErr = WaitForMCPConditionStatus(oc, mcpName, "Updated", "True", 7*time.Minute, 5*time.Second)
	o.Expect(mcpErr).NotTo(o.HaveOccurred(), fmt.Sprintf("%v MCP could not reach an updated state.", mcpName))
	return nil
}
