package mcpdeletion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	clientmachine "github.com/openshift/client-go/machine/clientset/versioned"
	informermachine "github.com/openshift/client-go/machine/informers/externalversions"
	listermachine "github.com/openshift/client-go/machine/listers/machine/v1beta1"
	mcfgclientset "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	informermcfgv1 "github.com/openshift/client-go/machineconfiguration/informers/externalversions/machineconfiguration/v1"
	listermcfgv1 "github.com/openshift/client-go/machineconfiguration/listers/machineconfiguration/v1"

	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
)

const (
	// maxRetries is the number of times a MCP will be retried before it is dropped out of the queue.
	maxRetries = 5

	// ControllerName is the name of this controller
	ControllerName = "MCPDeletionController"
)

// Controller manages MCP deletion protection finalizers
type Controller struct {
	mcfgClient    mcfgclientset.Interface
	kubeClient    clientset.Interface
	machineClient clientmachine.Interface

	mcpLister  listermcfgv1.MachineConfigPoolLister
	mcpsSynced cache.InformerSynced

	nodeLister     v1core.NodeInterface
	machineLister  listermachine.MachineLister
	machinesSynced cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

// NewController creates a new MCP deletion controller
func NewController(
	mcfgInformer informermcfgv1.MachineConfigPoolInformer,
	machineInformer informermachine.MachineInformer,
	kubeClient clientset.Interface,
	mcfgClient mcfgclientset.Interface,
	machineClient clientmachine.Interface,
) *Controller {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	ctrl := &Controller{
		mcfgClient:    mcfgClient,
		kubeClient:    kubeClient,
		machineClient: machineClient,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: ControllerName}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
	}

	mcpInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ctrl.addMachineConfigPool,
		UpdateFunc: ctrl.updateMachineConfigPool,
		DeleteFunc: ctrl.deleteMachineConfigPool,
	})
	ctrl.mcpLister = mcpInformer.Lister()
	ctrl.mcpsSynced = mcpInformer.Informer().HasSynced

	if machineInformer != nil {
		ctrl.machineLister = machineInformer.Lister()
		ctrl.machinesSynced = machineInformer.Informer().HasSynced
	}

	ctrl.nodeLister = kubeClient.CoreV1().Nodes()

	return ctrl
}

// Run starts the controller
func (ctrl *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ctrl.queue.ShutDown()

	klog.Infof("Starting %s", ControllerName)
	defer klog.Infof("Shutting down %s", ControllerName)

	if !cache.WaitForCacheSync(stopCh, ctrl.mcpsSynced) {
		return
	}

	// Wait for machine informer if available
	if ctrl.machinesSynced != nil {
		if !cache.WaitForCacheSync(stopCh, ctrl.machinesSynced) {
			return
		}
	}

	for i := 0; i < workers; i++ {
		go wait.Until(ctrl.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (ctrl *Controller) addMachineConfigPool(obj interface{}) {
	pool := obj.(*mcfgv1.MachineConfigPool)
	klog.V(4).Infof("Adding MachineConfigPool %s", pool.Name)
	ctrl.enqueueMachineConfigPool(pool)
}

func (ctrl *Controller) updateMachineConfigPool(old, cur interface{}) {
	oldPool := old.(*mcfgv1.MachineConfigPool)
	curPool := cur.(*mcfgv1.MachineConfigPool)
	klog.V(4).Infof("Updating MachineConfigPool %s", oldPool.Name)
	ctrl.enqueueMachineConfigPool(curPool)
}

func (ctrl *Controller) deleteMachineConfigPool(obj interface{}) {
	pool, ok := obj.(*mcfgv1.MachineConfigPool)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		pool, ok = tombstone.Obj.(*mcfgv1.MachineConfigPool)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a MachineConfigPool %#v", obj))
			return
		}
	}
	klog.V(4).Infof("Deleting MachineConfigPool %s", pool.Name)
	ctrl.enqueueMachineConfigPool(pool)
}

func (ctrl *Controller) enqueueMachineConfigPool(pool *mcfgv1.MachineConfigPool) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pool)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %w", pool, err))
		return
	}
	ctrl.queue.Add(key)
}

func (ctrl *Controller) worker() {
	for ctrl.processNextWorkItem() {
	}
}

func (ctrl *Controller) processNextWorkItem() bool {
	key, quit := ctrl.queue.Get()
	if quit {
		return false
	}
	defer ctrl.queue.Done(key)

	err := ctrl.syncHandler(key.(string))
	ctrl.handleErr(err, key)

	return true
}

func (ctrl *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		ctrl.queue.Forget(key)
		return
	}

	if ctrl.queue.NumRequeues(key) < maxRetries {
		klog.V(2).Infof("Error syncing MachineConfigPool %v: %v", key, err)
		ctrl.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(2).Infof("Dropping MachineConfigPool %q out of the queue: %v", key, err)
	ctrl.queue.Forget(key)
}

func (ctrl *Controller) syncHandler(key string) error {
	startTime := time.Now()
	klog.V(4).Infof("Started syncing MachineConfigPool %q (%v)", key, startTime)
	defer func() {
		klog.V(4).Infof("Finished syncing MachineConfigPool %q (%v)", key, time.Since(startTime))
	}()

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	pool, err := ctrl.mcpLister.Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).Infof("MachineConfigPool %v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	return ctrl.syncMachineConfigPool(pool.DeepCopy())
}

func (ctrl *Controller) syncMachineConfigPool(pool *mcfgv1.MachineConfigPool) error {
	// Skip default system pools (master, worker) - they shouldn't have deletion protection
	if isSystemPool(pool.Name) {
		return nil
	}

	// Handle deletion case
	if pool.DeletionTimestamp != nil {
		return ctrl.handleMCPDeletion(pool)
	}

	// Handle creation/update case - ensure finalizer is present
	return ctrl.ensureFinalizerPresent(pool)
}

// ensureFinalizerPresent adds the deletion protection finalizer to custom MCPs
func (ctrl *Controller) ensureFinalizerPresent(pool *mcfgv1.MachineConfigPool) error {
	finalizer := ctrlcommon.GetMCPDeletionFinalizer(pool.Name)

	// Check if finalizer is already present
	if ctrlcommon.InSlice(finalizer, pool.Finalizers) {
		return nil
	}

	klog.Infof("Adding deletion protection finalizer to custom MachineConfigPool %s", pool.Name)

	// Add the finalizer
	return ctrl.addFinalizerToMCP(pool, finalizer)
}

// handleMCPDeletion processes MCP deletion requests and validates safety
func (ctrl *Controller) handleMCPDeletion(pool *mcfgv1.MachineConfigPool) error {
	finalizer := ctrlcommon.GetMCPDeletionFinalizer(pool.Name)

	// If our finalizer is not present, nothing to do
	if !ctrlcommon.InSlice(finalizer, pool.Finalizers) {
		return nil
	}

	klog.Infof("Processing deletion request for custom MachineConfigPool %s", pool.Name)

	// Validate deletion safety
	canDelete, blockingReason, err := ctrl.validateMCPDeletionSafety(pool)
	if err != nil {
		return fmt.Errorf("failed to validate MCP deletion safety: %w", err)
	}

	if !canDelete {
		// Record event explaining why deletion is blocked
		ctrl.eventRecorder.Eventf(pool, corev1.EventTypeWarning, "DeletionBlocked",
			"Cannot delete MachineConfigPool %s: %s", pool.Name, blockingReason)

		klog.Warningf("Blocking deletion of MachineConfigPool %s: %s", pool.Name, blockingReason)

		// Requeue to check again later
		return fmt.Errorf("deletion blocked: %s", blockingReason)
	}

	// Safe to delete - remove our finalizer
	klog.Infof("MachineConfigPool %s is safe to delete, removing finalizer", pool.Name)
	ctrl.eventRecorder.Eventf(pool, corev1.EventTypeNormal, "DeletionAllowed",
		"MachineConfigPool %s is safe to delete", pool.Name)

	return ctrl.removeFinalizerFromMCP(pool, finalizer)
}

// validateMCPDeletionSafety implements the validation rules for MCP deletion
func (ctrl *Controller) validateMCPDeletionSafety(pool *mcfgv1.MachineConfigPool) (bool, string, error) {
	// Rule 1: Check if there are any machines still in this MCP
	hasActiveMachines, machineBlockingReason, err := ctrl.checkForActiveMachines(pool)
	if err != nil {
		return false, "", fmt.Errorf("failed to check for active machines: %w", err)
	}
	if hasActiveMachines {
		return false, machineBlockingReason, nil
	}

	// Rule 2: Check if any eligible nodes are still targeting this MCP's rendered config
	hasTargetingNodes, nodeBlockingReason, err := ctrl.checkForTargetingNodes(pool)
	if err != nil {
		return false, "", fmt.Errorf("failed to check for targeting nodes: %w", err)
	}
	if hasTargetingNodes {
		return false, nodeBlockingReason, nil
	}

	return true, "", nil
}

// checkForActiveMachines implements Rule 1: Check for machines still in this MCP
func (ctrl *Controller) checkForActiveMachines(pool *mcfgv1.MachineConfigPool) (bool, string, error) {
	// TODO: IMPLEMENT MACHINE CHECKING LOGIC
	//
	// This function should:
	// 1. List all machines in the openshift-machine-api namespace
	// 2. Check if any machines are configured to create nodes for this MCP
	// 3. Machines target MCPs through their machine role labels and node selectors
	//
	// Implementation steps:
	// - Use ctrl.machineClient.MachineV1beta1().Machines("openshift-machine-api").List()
	// - For each machine, check if machine.Labels["machine.openshift.io/cluster-api-machine-role"] == pool.Name
	// - OR check if the machine's providerSpec would result in nodes matching this pool's nodeSelector
	//
	// Example implementation:
	/*
		machines, err := ctrl.machineClient.MachineV1beta1().Machines("openshift-machine-api").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, "", fmt.Errorf("failed to list machines: %w", err)
		}

		var blockingMachines []string
		for _, machine := range machines.Items {
			// Check if machine role matches this pool
			if machine.Labels != nil {
				if role, exists := machine.Labels["machine.openshift.io/cluster-api-machine-role"]; exists && role == pool.Name {
					blockingMachines = append(blockingMachines, machine.Name)
					continue
				}
			}

			// TODO: Also check providerSpec for node labels that would match this pool's nodeSelector
			// This is more complex and requires parsing the machine's providerSpec
		}

		if len(blockingMachines) > 0 {
			return true, fmt.Sprintf("Machines still configured for this pool: %v", blockingMachines), nil
		}
	*/

	klog.Warningf("TODO: Machine checking not yet implemented for MCP %s", pool.Name)
	return false, "", nil
}

// checkForTargetingNodes implements Rule 2: Check for nodes still targeting this MCP's rendered config
func (ctrl *Controller) checkForTargetingNodes(pool *mcfgv1.MachineConfigPool) (bool, string, error) {
	// TODO: IMPLEMENT NODE TARGETING CHECK LOGIC
	//
	// This function should:
	// 1. List all nodes in the cluster
	// 2. Filter to eligible nodes (non-master, non-arbiter)
	// 3. Check if any eligible nodes are still targeting this MCP's rendered config
	//
	// Implementation steps:
	// - Use ctrl.kubeClient.CoreV1().Nodes().List() or ctrl.nodeLister
	// - For each node, check eligibility:
	//   * Skip nodes with label "node-role.kubernetes.io/master"
	//   * Skip nodes with label "node-role.kubernetes.io/arbiter"
	// - Check node annotations for current/desired config:
	//   * daemonconsts.CurrentMachineConfigAnnotationKey (machineconfiguration.openshift.io/currentConfig)
	//   * daemonconsts.DesiredMachineConfigAnnotationKey (machineconfiguration.openshift.io/desiredConfig)
	// - Check if these configs match the pattern "rendered-{poolName}-*"
	//
	// Example implementation:
	/*
		nodes, err := ctrl.kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, "", fmt.Errorf("failed to list nodes: %w", err)
		}

		renderedConfigPrefix := "rendered-" + pool.Name + "-"
		var blockingNodes []string

		for _, node := range nodes.Items {
			// Skip master and arbiter nodes (not eligible for custom MCPs)
			if _, isMaster := node.Labels["node-role.kubernetes.io/master"]; isMaster {
				continue
			}
			if _, isArbiter := node.Labels["node-role.kubernetes.io/arbiter"]; isArbiter {
				continue
			}

			// Check current config annotation
			if currentConfig, exists := node.Annotations[daemonconsts.CurrentMachineConfigAnnotationKey]; exists {
				if strings.HasPrefix(currentConfig, renderedConfigPrefix) {
					blockingNodes = append(blockingNodes, fmt.Sprintf("%s (current: %s)", node.Name, currentConfig))
					continue
				}
			}

			// Check desired config annotation
			if desiredConfig, exists := node.Annotations[daemonconsts.DesiredMachineConfigAnnotationKey]; exists {
				if strings.HasPrefix(desiredConfig, renderedConfigPrefix) {
					blockingNodes = append(blockingNodes, fmt.Sprintf("%s (desired: %s)", node.Name, desiredConfig))
					continue
				}
			}
		}

		if len(blockingNodes) > 0 {
			return true, fmt.Sprintf("Eligible nodes still targeting this pool's config: %v", blockingNodes), nil
		}
	*/

	klog.Warningf("TODO: Node targeting check not yet implemented for MCP %s", pool.Name)
	return false, "", nil
}

// Helper functions for finalizer management

func (ctrl *Controller) addFinalizerToMCP(pool *mcfgv1.MachineConfigPool, finalizer string) error {
	// Create patch to add finalizer
	poolCopy := pool.DeepCopy()
	poolCopy.Finalizers = append(poolCopy.Finalizers, finalizer)

	return ctrl.patchMCP(pool, poolCopy)
}

func (ctrl *Controller) removeFinalizerFromMCP(pool *mcfgv1.MachineConfigPool, finalizer string) error {
	// Create patch to remove finalizer
	poolCopy := pool.DeepCopy()
	finalizers := []string{}
	for _, f := range poolCopy.Finalizers {
		if f != finalizer {
			finalizers = append(finalizers, f)
		}
	}
	poolCopy.Finalizers = finalizers

	return ctrl.patchMCP(pool, poolCopy)
}

func (ctrl *Controller) patchMCP(original, modified *mcfgv1.MachineConfigPool) error {
	originalJSON, err := json.Marshal(original)
	if err != nil {
		return err
	}

	modifiedJSON, err := json.Marshal(modified)
	if err != nil {
		return err
	}

	patch, err := createMergePatch(originalJSON, modifiedJSON)
	if err != nil {
		return err
	}

	if len(patch) == 0 || string(patch) == "{}" {
		return nil // No changes needed
	}

	_, err = ctrl.mcfgClient.MachineconfigurationV1().MachineConfigPools().Patch(
		context.TODO(),
		original.Name,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)

	return err
}

// Helper functions

func isSystemPool(poolName string) bool {
	return poolName == "master" || poolName == "worker"
}

// createMergePatch creates a strategic merge patch
func createMergePatch(original, modified []byte) ([]byte, error) {
	// Simple implementation - in production you might want to use a proper 3-way merge
	// For now, just return the modified object as a merge patch
	var originalMap, modifiedMap map[string]interface{}

	if err := json.Unmarshal(original, &originalMap); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(modified, &modifiedMap); err != nil {
		return nil, err
	}

	// Create patch with only the finalizers field for simplicity
	patchMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": modifiedMap["metadata"].(map[string]interface{})["finalizers"],
		},
	}

	return json.Marshal(patchMap)
}
