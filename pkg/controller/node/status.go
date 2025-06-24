package node

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	features "github.com/openshift/api/features"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/openshift/machine-config-operator/pkg/apihelpers"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	daemonconsts "github.com/openshift/machine-config-operator/pkg/daemon/constants"
	helpers "github.com/openshift/machine-config-operator/pkg/helpers"
)

// syncStatusOnly for MachineConfigNode
func (ctrl *Controller) syncStatusOnly(pool *mcfgv1.MachineConfigPool) error {
	cc, err := ctrl.ccLister.Get(ctrlcommon.ControllerConfigName)
	if err != nil {
		return err
	}
	nodes, err := ctrl.getNodesForPool(pool)
	if err != nil {
		return err
	}

	machineConfigStates := []*mcfgv1.MachineConfigNode{}
	if ctrl.fgHandler.Enabled(features.FeatureGateMachineConfigNodes) {
		for _, node := range nodes {
			ms, err := ctrl.client.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Could not find our MachineConfigNode for node. %s: %v", node.Name, err)
				continue
			}
			machineConfigStates = append(machineConfigStates, ms)
		}
	}

	mosc, mosb, l, err := ctrl.getConfigAndBuildAndLayeredStatus(pool)
	if err != nil {
		return fmt.Errorf("could get MachineOSConfig or MachineOSBuild: %w", err)
	}

	newStatus := ctrl.calculateStatus(machineConfigStates, cc, pool, nodes, mosc, mosb)
	if equality.Semantic.DeepEqual(pool.Status, newStatus) {
		return nil
	}

	newPool := pool
	newPool.Status = newStatus
	_, err = ctrl.client.MachineconfigurationV1().MachineConfigPools().UpdateStatus(context.TODO(), newPool, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("could not update MachineConfigPool %q: %w", newPool.Name, err)
	}

	if pool.Spec.Configuration.Name != newPool.Spec.Configuration.Name {
		ctrl.eventRecorder.Eventf(pool, corev1.EventTypeNormal, "Updating", "Pool %s now targeting %s", pool.Name, getPoolUpdateLine(newPool, mosc, l))
	}
	if pool.Status.Configuration.Name != newPool.Status.Configuration.Name {
		ctrl.eventRecorder.Eventf(pool, corev1.EventTypeNormal, "Completed", "Pool %s has completed update to %s", pool.Name, getPoolUpdateLine(newPool, mosc, l))
	}
	return err
}

// `calculateStatus` calculates the MachineConfigPoolStatus object for the desired MCP
//
//nolint:gocyclo,gosec
func (ctrl *Controller) calculateStatus(mcns []*mcfgv1.MachineConfigNode, cconfig *mcfgv1.ControllerConfig, pool *mcfgv1.MachineConfigPool, nodes []*corev1.Node, mosc *mcfgv1.MachineOSConfig, mosb *mcfgv1.MachineOSBuild) mcfgv1.MachineConfigPoolStatus {
	certExpirys := []mcfgv1.CertExpiry{}
	if cconfig != nil {
		for _, cert := range cconfig.Status.ControllerCertificates {
			if cert.BundleFile == "KubeAPIServerServingCAData" {
				certExpirys = append(certExpirys, mcfgv1.CertExpiry{
					Bundle:  cert.BundleFile,
					Subject: cert.Subject,
					Expiry:  cert.NotAfter,
				},
				)
			}
		}
	}

	// Get total machine count and initialize pool synchronizer for MCP
	totalMachineCount := int32(len(nodes))
	poolSynchronizer := newPoolSynchronizer(totalMachineCount)

	// Determine if pool is layered and enabled feature gates
	isLayeredPool := ctrl.isLayeredPool(mosc, mosb)
	pisIsEnabled := ctrl.fgHandler.Enabled(features.FeatureGatePinnedImages)
	imageModeReportingIsEnabled := ctrl.fgHandler.Enabled(features.FeatureGateImageModeStatusReporting)

	// Update the number of degraded, updated, and updating machines from conditions in the MCNs
	// for the nodes in the associated MCP.
	var degradedMachines, updatedMachines, updatingMachines []*corev1.Node
	degradedReasons := []string{}
	pinnedImageSetsDegraded := false
	for _, mcn := range mcns {
		// Get the node associated with the MCN
		var ourNode *corev1.Node
		for _, n := range nodes {
			if mcn.Name == n.Name {
				ourNode = n
				break
			}
		}
		if ourNode == nil {
			klog.Errorf("Could not find specified node %s", mcn.Name)
		}

		// If the MCN Conditions list is empty, the MCN is not ready and cannot be used to determine the MCP status
		if len(mcn.Status.Conditions) == 0 {
			// not ready yet
			break
		}

		// If the PIS feature gate is enabled, update the PIS reference in the PoolSynchronizer object
		if pisIsEnabled {
			if isPinnedImageSetsUpdated(mcn) {
				poolSynchronizer.SetUpdated(mcfgv1.PinnedImageSets)
			}
		}

		// Loop through the MCN conditions to determine if the associated node is updating, updated, or degraded
		for _, cond := range mcn.Status.Conditions {
			// populate the degradedReasons from the MachineConfigNodeNodeDegraded condition
			if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodeNodeDegraded && cond.Status == metav1.ConditionTrue {
				degradedMachines = append(degradedMachines, ourNode)
				degradedReasons = append(degradedReasons, fmt.Sprintf("Node %s is reporting: %q", ourNode.Name, cond.Message))
				// TODO: understand if node can be updating or updated in addition to being degraded
				break
			}
			/*
				// TODO (ijanssen): This section of code should be implemented as part of OCPBUGS-57177 after OCPBUGS-32745 is addressed.
				// 	In the current state of the code, the `MachineConfigNodePinnedImageSetsDegraded` condition is wrongly being set to True`
				// 	even when no unintended functionality is occurring, such as many images taking more than 2 minutes to fetch. Thus, it is
				//  not wise to degrade an MCP on a PIS degrade while the PIS degrade is not acting as intended. OCPBUGS-57177 has
				// 	been marked as blocked by OCPBUGS-32745 in Jira, but once the degrade condition is stablilized, this code block should
				// 	cover the fix for OCPBUGS-57177.
					// populate the degradedReasons from the MachineConfigNodePinnedImageSetsDegraded condition
					if pisIsEnabled && mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodePinnedImageSetsDegraded && cond.Status == metav1.ConditionTrue {
						degradedMachines = append(degradedMachines, ourNode)
						degradedReasons = append(degradedReasons, fmt.Sprintf("Node %s references an invalid PinnedImageSet. See the node's MachineConfigNode resource for details.", ourNode.Name))
						if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodePinnedImageSetsDegraded {
							pinnedImageSetsDegraded = true
						}
						break
					}
			*/

			// If the ImageModeStatusReporting feature gate is enabled, the updating and updated machine
			// counts in the MCP status should be populated from MCN conditions
			if imageModeReportingIsEnabled {
				// A node is considered "updated" when the following are true:
				// 	- The desired and current config versions and current and desired images are equal,
				// 	  which is only met in the MCN when the `Updated` status is `True`.
				// 	- The MCN's desired config version matches the MCP's desired config version. Note
				// 	  that this check is required to ensure no regressions occur in migrating to the
				// 	  MCN driven MCP updates.
				if mcfgv1.StateProgress(cond.Type) == mcfgv1.MachineConfigNodeUpdated && cond.Status == metav1.ConditionTrue &&
					mcn.Spec.ConfigVersion.Desired == pool.Spec.Configuration.Name {
					updatedMachines = append(updatedMachines, ourNode)
				} else {
					updatingMachines = append(updatingMachines, ourNode)
				}
				break
			}
		}
	}

	// Calculate degraded, updated, and updating machine counts as determined by the MCN conditions,
	// and get the total machine count.
	degradedMachineCount := int32(len(degradedMachines))
	updatedMachineCount := int32(len(updatedMachines))
	updatingMachineCount := int32(len(updatingMachines))

	// TODO (ijanssen): once we are comfortable with the implementation, we can probably remove this check
	// TODO: understand if a machine can be considered updating & degraded?
	// In a standard update case, the total number of machines should equal the sum of the updating and updated nodes.
	// However, when one or more machines is degraded, the non-degraded machines may not be condidered updated or
	// updating, so the different machine statuses will not total the number of nodes.
	if (imageModeReportingIsEnabled && updatedMachineCount+updatingMachineCount != totalMachineCount && degradedMachineCount == 0) || // handle case when ImageModeStatusReporting is enabled & machine counts do not reconcile
		!imageModeReportingIsEnabled { // use historically existing functionality when ImageModeStatusReporting is not enabled
		// When we get here and image mode status reporting is enabled, it means the machine counts were determined
		// incorrectly. Recording an event means we can track that the functionality is not implemented correctly.
		if imageModeReportingIsEnabled {
			ctrl.eventRecorder.Eventf(pool, corev1.EventTypeWarning, "MachineCountFail", "ImageModeStatusReporting feature gate is enabled and machine counts did not reconcile for MCP %s. Got updated count of %v, updating count of %v, and degraded count of %v.", pool.Name, updatedMachineCount, updatingMachineCount, degradedMachineCount)
		}

		// Get correct updated & degraded machine counts
		updatedMachines = getUpdatedMachines(pool, nodes, mosc, mosb, isLayeredPool)
		updatedMachineCount = int32(len(updatedMachines))
		degradedMachines = getDegradedMachines(nodes)
		degradedMachineCount = int32(len(degradedMachines))
	} else { // TODO: remove post debugging
		ctrl.eventRecorder.Eventf(pool, corev1.EventTypeNormal, "MachineCountSuccess", "ImageModeStatusReporting feature gate is enabled and machine counts were correctly determined for MCP %s.", pool.Name)
	}

	// Get ready & unavailable machine counts
	// Note that since the MCN does not, contain kubelet information for a node, as of implementing
	// MCO-1506 for 4.20, the ready and unavailable machine counts cannot be determined by their
	// condition information. Rather, it must be determined by node properties.
	readyMachines := getReadyMachines(pool, nodes, mosc, mosb, isLayeredPool)
	readyMachineCount := int32(len(readyMachines))
	unavailableMachines := getUnavailableMachines(nodes, pool)
	unavailableMachineCount := int32(len(unavailableMachines))

	// Update MCP status with machine counts
	status := mcfgv1.MachineConfigPoolStatus{
		ObservedGeneration:      pool.Generation,
		MachineCount:            totalMachineCount,
		UpdatedMachineCount:     updatedMachineCount,
		ReadyMachineCount:       readyMachineCount,
		UnavailableMachineCount: unavailableMachineCount,
		DegradedMachineCount:    degradedMachineCount,
		CertExpirys:             certExpirys,
	}

	// update synchronizer status for pinned image sets
	if pisIsEnabled {
		syncStatus := poolSynchronizer.GetStatus(mcfgv1.PinnedImageSets)
		status.PoolSynchronizersStatus = []mcfgv1.PoolSynchronizerStatus{
			{
				PoolSynchronizerType:    mcfgv1.PinnedImageSets,
				MachineCount:            syncStatus.MachineCount,
				UpdatedMachineCount:     syncStatus.UpdatedMachineCount,
				ReadyMachineCount:       int64(readyMachineCount),
				UnavailableMachineCount: int64(unavailableMachineCount),
				AvailableMachineCount:   int64(totalMachineCount - unavailableMachineCount),
			},
		}
	}

	// Update MCP status configuation & conditions
	status.Configuration = pool.Status.Configuration
	conditions := pool.Status.Conditions
	status.Conditions = append(status.Conditions, conditions...)

	// Determine if all machines are update
	// 	- If all machines are updated, set "Updated" condition to true and "Updating" condition to false
	// 	- If all machines not updated, set "Updated" condition to false and "Updating" condition to false
	// 	  if the pool is paused or true if the pool is not paused and the PIS is not degraded
	allUpdated := updatedMachineCount == totalMachineCount &&
		readyMachineCount == totalMachineCount &&
		unavailableMachineCount == 0
	if allUpdated {
		//TODO: update api to only have one condition regarding status of update.
		updatedMsg := fmt.Sprintf("All nodes are updated with %s", getPoolUpdateLine(pool, mosc, isLayeredPool))
		supdated := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolUpdated, corev1.ConditionTrue, "", updatedMsg)
		apihelpers.SetMachineConfigPoolCondition(&status, *supdated)

		supdating := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolUpdating, corev1.ConditionFalse, "", "")
		apihelpers.SetMachineConfigPoolCondition(&status, *supdating)
		if status.Configuration.Name != pool.Spec.Configuration.Name || !equality.Semantic.DeepEqual(status.Configuration.Source, pool.Spec.Configuration.Source) {
			klog.Infof("Pool %s: %s", pool.Name, updatedMsg)
			status.Configuration = pool.Spec.Configuration
		}
	} else {
		supdated := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolUpdated, corev1.ConditionFalse, "", "")
		apihelpers.SetMachineConfigPoolCondition(&status, *supdated)
		if pool.Spec.Paused {
			supdating := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolUpdating, corev1.ConditionFalse, "", fmt.Sprintf("Pool is paused; will not update to %s", getPoolUpdateLine(pool, mosc, isLayeredPool)))
			apihelpers.SetMachineConfigPoolCondition(&status, *supdating)
		} else if !pinnedImageSetsDegraded { // note that when the PinnedImageSet is degraded, the `Updating` status should not be updated
			supdating := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolUpdating, corev1.ConditionTrue, "", fmt.Sprintf("All nodes are updating to %s", getPoolUpdateLine(pool, mosc, isLayeredPool)))
			apihelpers.SetMachineConfigPoolCondition(&status, *supdating)
		}
	}

	// If the MachineOSConfig is not nil, image mode is enabled and the MCP status should be handled
	// according to its and the MachineOSBuild's statuses
	if mosc != nil && !pool.Spec.Paused {
		// MOSC exists but MOSB doesn't exist yet -> change MCP to updating
		if mosb == nil {
			updating := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolUpdating, corev1.ConditionTrue, "", fmt.Sprintf("Pool is waiting for a new OS image build to start (mosc: %s)", mosc.Name))
			apihelpers.SetMachineConfigPoolCondition(&status, *updating)
		} else {
			// Some cases we have an old MOSB object that still exists, we still update MCP
			mosbState := ctrlcommon.NewMachineOSBuildState(mosb)
			if mosbState.IsBuilding() || mosbState.IsBuildPrepared() {
				updating := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolUpdating, corev1.ConditionTrue, "", fmt.Sprintf("Pool is waiting for OS image build to complete (mosb: %s)", mosb.Name))
				apihelpers.SetMachineConfigPoolCondition(&status, *updating)
			}
		}
	}

	// Create degrade message by aggregating degraded reasons from all degraded machines
	for _, n := range degradedMachines {
		reason, ok := n.Annotations[daemonconsts.MachineConfigDaemonReasonAnnotationKey]
		if ok && reason != "" {
			degradedReasons = append(degradedReasons, fmt.Sprintf("Node %s is reporting: %q", n.Name, reason))
		}
	}

	// Update degrade condition
	var nodeDegraded bool
	var nodeDegradedMessage string
	for _, m := range degradedMachines {
		klog.Infof("Degraded Machine: %v and Degraded Reason: %v", m.Name, m.Annotations[daemonconsts.MachineConfigDaemonReasonAnnotationKey])
	}
	if degradedMachineCount > 0 {
		nodeDegraded = true
		sdegraded := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolNodeDegraded, corev1.ConditionTrue, fmt.Sprintf("%d nodes are reporting degraded status on sync", len(degradedMachines)), strings.Join(degradedReasons, ", "))
		nodeDegradedMessage = sdegraded.Message
		apihelpers.SetMachineConfigPoolCondition(&status, *sdegraded)
		if pinnedImageSetsDegraded {
			sdegraded := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolPinnedImageSetsDegraded, corev1.ConditionTrue, "one or more pinned image set is reporting degraded", strings.Join(degradedReasons, ", "))
			apihelpers.SetMachineConfigPoolCondition(&status, *sdegraded)
		}
	} else { // TODO: ijanssen: see if render degraded makes things degraded still even though the degraded machine count is not > 0
		sdegraded := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolNodeDegraded, corev1.ConditionFalse, "", "")
		apihelpers.SetMachineConfigPoolCondition(&status, *sdegraded)
	}

	// here we now set the MCP Degraded field, the node_controller is the one making the call right now
	// but we might have a dedicated controller or control loop somewhere else that understands how to
	// set Degraded. For now, the node_controller understand NodeDegraded & RenderDegraded = Degraded.
	renderDegraded := apihelpers.IsMachineConfigPoolConditionTrue(pool.Status.Conditions, mcfgv1.MachineConfigPoolRenderDegraded)
	if nodeDegraded || renderDegraded || pinnedImageSetsDegraded {
		sdegraded := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolDegraded, corev1.ConditionTrue, "", "")
		if nodeDegraded {
			sdegraded.Message = nodeDegradedMessage
		}
		apihelpers.SetMachineConfigPoolCondition(&status, *sdegraded)

	} else {
		sdegraded := apihelpers.NewMachineConfigPoolCondition(mcfgv1.MachineConfigPoolDegraded, corev1.ConditionFalse, "", "")
		apihelpers.SetMachineConfigPoolCondition(&status, *sdegraded)
	}

	return status
}

func getPoolUpdateLine(pool *mcfgv1.MachineConfigPool, mosc *mcfgv1.MachineOSConfig, layered bool) string {
	targetConfig := pool.Spec.Configuration.Name
	mcLine := fmt.Sprintf("MachineConfig %s", targetConfig)

	if !layered {
		return mcLine
	}

	targetImage := mosc.Status.CurrentImagePullSpec
	if string(targetImage) == "" {
		return mcLine
	}

	return fmt.Sprintf("%s / Image %s", mcLine, targetImage)
}

// isNodeManaged checks whether the MCD has ever run on a node
func isNodeManaged(node *corev1.Node) bool {
	if helpers.IsWindows(node) {
		klog.V(4).Infof("Node %v is a windows node so won't be managed by MCO", node.Name)
		return false
	}
	if node.Annotations == nil {
		return false
	}
	cconfig, ok := node.Annotations[daemonconsts.CurrentMachineConfigAnnotationKey]
	if !ok || cconfig == "" {
		return false
	}
	return true
}

// getUpdatedMachines filters the provided nodes to return the nodes whose
// current config matches the desired config, which also matches the target config,
// and the "done" flag is set.
func getUpdatedMachines(pool *mcfgv1.MachineConfigPool, nodes []*corev1.Node, mosc *mcfgv1.MachineOSConfig, mosb *mcfgv1.MachineOSBuild, layered bool) []*corev1.Node {
	var updated []*corev1.Node
	for _, node := range nodes {
		lns := ctrlcommon.NewLayeredNodeState(node)
		if lns.IsDone(pool, layered, mosc, mosb) {
			updated = append(updated, node)
		}
	}
	return updated
}

// getReadyMachines filters the provided nodes to return the nodes
// that are updated and marked ready
func getReadyMachines(pool *mcfgv1.MachineConfigPool, nodes []*corev1.Node, mosc *mcfgv1.MachineOSConfig, mosb *mcfgv1.MachineOSBuild, layered bool) []*corev1.Node {
	updated := getUpdatedMachines(pool, nodes, mosc, mosb, layered)
	var ready []*corev1.Node
	for _, node := range updated {
		lns := ctrlcommon.NewLayeredNodeState(node)
		if lns.IsNodeReady() {
			ready = append(ready, node)
		}
	}
	return ready
}

// getUnavailableMachines returns the set of nodes which are
// either marked unscheduleable, or have a MCD actively working.
// If the MCD is actively working (or hasn't started) then the
// node *may* go unschedulable in the future, so we don't want to
// potentially start another node update exceeding our maxUnavailable.
// Somewhat the opposite of getReadyNodes().
func getUnavailableMachines(nodes []*corev1.Node, pool *mcfgv1.MachineConfigPool) []*corev1.Node {
	var unavail []*corev1.Node
	for _, node := range nodes {
		lns := ctrlcommon.NewLayeredNodeState(node)
		if lns.IsUnavailableForUpdate() {
			unavail = append(unavail, node)
			klog.V(4).Infof("getUnavailableMachines: Found unavailable node %s in pool %s", node.Name, pool.Name)
		}
	}
	klog.V(4).Infof("getUnavailableMachines: Found %d unavailable in pool %s", len(unavail), pool.Name)
	return unavail
}

func getDegradedMachines(nodes []*corev1.Node) []*corev1.Node {
	var degraded []*corev1.Node
	for _, node := range nodes {
		if node.Annotations == nil {
			continue
		}
		dconfig, ok := node.Annotations[daemonconsts.DesiredMachineConfigAnnotationKey]
		if !ok || dconfig == "" {
			continue
		}
		dstate, ok := node.Annotations[daemonconsts.MachineConfigDaemonStateAnnotationKey]
		if !ok || dstate == "" {
			continue
		}

		if dstate == daemonconsts.MachineConfigDaemonStateDegraded || dstate == daemonconsts.MachineConfigDaemonStateUnreconcilable {
			degraded = append(degraded, node)
		}
	}
	return degraded
}

func getNamesFromNodes(nodes []*corev1.Node) []string {
	if len(nodes) == 0 {
		return nil
	}

	names := []string{}
	for _, node := range nodes {
		names = append(names, node.Name)
	}

	return names
}

// newPoolSynchronizer creates a new pool synchronizer.
func newPoolSynchronizer(machineCount int32) *poolSynchronizer {
	return &poolSynchronizer{
		synchronizers: map[mcfgv1.PoolSynchronizerType]*mcfgv1.PoolSynchronizerStatus{
			mcfgv1.PinnedImageSets: {
				PoolSynchronizerType: mcfgv1.PinnedImageSets,
				MachineCount:         int64(machineCount),
			},
		},
	}
}

// poolSynchronizer is a helper struct to track the status of multiple synchronizers for a pool.
type poolSynchronizer struct {
	synchronizers map[mcfgv1.PoolSynchronizerType]*mcfgv1.PoolSynchronizerStatus
}

// SetUpdated updates the updated machine count for the given synchronizer type.
func (p *poolSynchronizer) SetUpdated(sType mcfgv1.PoolSynchronizerType) {
	status := p.synchronizers[sType]
	if status == nil {
		return
	}
	status.UpdatedMachineCount++
}

func (p *poolSynchronizer) GetStatus(sType mcfgv1.PoolSynchronizerType) *mcfgv1.PoolSynchronizerStatus {
	return p.synchronizers[sType]
}

// isPinnedImageSetNodeUpdated checks if the pinned image sets are updated for the node.
func isPinnedImageSetsUpdated(mcn *mcfgv1.MachineConfigNode) bool {
	updated := 0
	for _, set := range mcn.Status.PinnedImageSets {
		if set.DesiredGeneration > 0 && set.CurrentGeneration == set.DesiredGeneration {
			updated++
		}
	}
	return updated == len(mcn.Status.PinnedImageSets)
}

// isPinnedImageSetsInProgressForPool checks if the pinned image sets are in progress of reconciling for the pool.
func isPinnedImageSetsInProgressForPool(pool *mcfgv1.MachineConfigPool) bool {
	for _, status := range pool.Status.PoolSynchronizersStatus {
		if status.PoolSynchronizerType == mcfgv1.PinnedImageSets && status.UpdatedMachineCount != status.MachineCount {
			return true
		}
	}
	return false
}
