// TODO (MCO-1960): Deduplicate these functions with the helpers defined in /extended-priv/node.go.
package extended

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"time"

	o "github.com/onsi/gomega"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	"github.com/openshift/machine-config-operator/pkg/daemon/constants"
	exutil "github.com/openshift/machine-config-operator/test/extended-priv/util"
	logger "github.com/openshift/machine-config-operator/test/extended-priv/util/logext"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// `GetNodesByRole` gets all nodes labeled with the desired role
func GetNodesByRole(oc *exutil.CLI, role string) ([]corev1.Node, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{fmt.Sprintf("node-role.kubernetes.io/%s", role): ""}).String(),
	}
	nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

// `GetAllNodes` gets all nodes from a cluster
func GetAllNodes(oc *exutil.CLI) ([]corev1.Node, error) {
	nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

func findNodeCondition(status []corev1.NodeCondition, name corev1.NodeConditionType, position int) *corev1.NodeCondition {
	if position < len(status) {
		if status[position].Type == name {
			return &status[position]
		}
	}
	for i := range status {
		if status[i].Type == name {
			return &status[i]
		}
	}
	return nil
}

// `isNodeKubeletReady` determines if a given node's kubelet is ready
func isNodeKubeletReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Reason == "KubeletReady" && condition.Status == "True" && condition.Type == "Ready" {
			return true
		}
	}

	return false
}

// `checkMCDState` determines whether the MCD state matches the provided desired state
func checkMCDState(node corev1.Node, desiredState string) bool {
	state := node.Annotations["machineconfiguration.openshift.io/state"]
	return state == desiredState
}

// `isNodeReady` determines if a given node is ready
func IsNodeReady(node corev1.Node) bool {
	// If the node is cordoned, it is not ready.
	if node.Spec.Unschedulable {
		return false
	}

	// If the nodes' kubelet is not ready, it is not ready.
	if !isNodeKubeletReady(node) {
		return false
	}

	// If the nodes' MCD is not done, it is not ready.
	if !checkMCDState(node, "Done") {
		return false
	}

	return true
}

// `GetRandomNode` gets a random node from with a given role and checks whether the node is ready. If no
// nodes are ready, it will wait for up to 5 minutes for a node to become available.
func GetRandomNode(oc *exutil.CLI, role string) corev1.Node {
	if node := getRandomNode(oc, role); IsNodeReady(node) {
		return node
	}

	// If no nodes are ready, wait for up to 5 minutes for one to be ready
	waitPeriod := time.Minute * 5
	logger.Infof("No ready nodes found with role '%s', waiting up to %s for a ready node to become available", role, waitPeriod)
	var targetNode corev1.Node
	o.Eventually(func() bool {
		if node := getRandomNode(oc, role); IsNodeReady(node) {
			targetNode = node
			return true
		}

		return false
	}, 5*time.Minute, 2*time.Second).Should(o.BeTrue())

	return targetNode
}

// `getRandomNode` gets a random node with a given role
func getRandomNode(oc *exutil.CLI, role string) corev1.Node {
	nodes, err := GetNodesByRole(oc, role)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodes).ShouldNot(o.BeEmpty())

	// Disable gosec here to avoid throwing
	// G404: Use of weak random number generator (math/rand instead of crypto/rand)
	// #nosec
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return nodes[rnd.Intn(len(nodes))]
}

// `WaitForMCPConditionStatus` waits up to the desired timeout for the desired MCP condition to match the desired status (ex. wait until "Updating" is "True")
func WaitForMCPConditionStatus(oc *exutil.CLI, mcpName string, conditionType mcfgv1.MachineConfigPoolConditionType, status corev1.ConditionStatus, timeout time.Duration, interval time.Duration) error {
	logger.Infof("Waiting up to %v for MCP '%s' condition '%s' to be '%s'.", timeout, mcpName, conditionType, status)
	machineConfigClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Eventually(func() bool {
		logger.Infof("Waiting for '%v' MCP's '%v' condition to be '%v'.", mcpName, conditionType, status)

		// Get MCP
		mcp, mcpErr := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), mcpName, metav1.GetOptions{})
		if mcpErr != nil {
			logger.Infof("Failed to grab MCP '%v', error :%v", mcpName, err)
			return false
		}

		// Loop through conditions to get check for desired condition type/status combination
		conditions := mcp.Status.Conditions
		for _, condition := range conditions {
			if condition.Type == conditionType {
				logger.Infof("MCP '%s' condition '%s' status is '%s'", mcp.Name, conditionType, condition.Status)
				return condition.Status == status
			}
		}

		return false
	}, timeout, interval).Should(o.BeTrue())
	return nil
}

// `GetUpdatingNode` returns the updating node, determined by the node targetting a new desired
// config, when the corresponding MCP starts updating
func GetUpdatingNode(oc *exutil.CLI, mcpName, originalConfigVersion string) corev1.Node {
	// Wait for the MCP to start updating
	o.Expect(WaitForMCPConditionStatus(oc, mcpName, mcfgv1.MachineConfigPoolUpdating, corev1.ConditionTrue, 3*time.Minute, 2*time.Second)).NotTo(o.HaveOccurred(), "Waiting for 'Updating' status change failed.")

	// Get first updating node & return it
	var updatingNode corev1.Node
	o.Eventually(func() bool {
		logger.Infof("Trying to get updating node in '%v' MCP.", mcpName)

		// Get nodes in MCP
		nodes, nodeErr := GetNodesByRole(oc, mcpName)
		o.Expect(nodeErr).NotTo(o.HaveOccurred(), "Error getting nodes from %v MCP.", mcpName)
		o.Expect(nodes).ShouldNot(o.BeEmpty(), "No nodes found for %v MCP.", mcpName)

		// Loop through nodes to see which is targetting a new desired config version
		for _, node := range nodes {
			if node.Annotations["machineconfiguration.openshift.io/desiredConfig"] != originalConfigVersion {
				updatingNode = node
				return true
			}
		}

		return false
	}, 30*time.Second, 1*time.Second).Should(o.BeTrue())

	return updatingNode
}

// `restoreDesiredConfig` updates the value of a node's desiredConfig annotation to be equal to the value of its currentConfig (desiredConfig=currentConfig)
func restoreDesiredConfig(oc *exutil.CLI, node corev1.Node) error {
	// Get current config
	currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
	if currentConfig == "" {
		return fmt.Errorf("currentConfig annotation is empty for node %s", node.Name)
	}

	// Update desired config to be equal to current config
	logger.Infof("Node: %s is restoring desiredConfig value to match currentConfig value: %s", node.Name, currentConfig)
	configErr := oc.Run("patch").Args(fmt.Sprintf("node/%v", node.Name), "--patch", fmt.Sprintf(`{"metadata":{"annotations":{"machineconfiguration.openshift.io/desiredConfig":"%v"}}}`, currentConfig), "--type=merge").Execute()
	return configErr
}

// `WaitForNodeCurrentConfig` waits up to 5 minutes for a input node to have a current
// config equal to the `config` parameter
func WaitForNodeCurrentConfig(oc *exutil.CLI, nodeName, config string) {
	o.Eventually(func() bool {
		node, nodeErr := oc.AsAdmin().KubeClient().CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if nodeErr != nil {
			logger.Infof("Failed to get node '%v', error :%v", nodeName, nodeErr)
			return false
		}

		// Check if the node's current config matches the input config version
		nodeCurrentConfig := node.Annotations[constants.CurrentMachineConfigAnnotationKey]
		if nodeCurrentConfig == config {
			logger.Infof("Node '%v' has successfully updated and has a current config version of '%v'.", nodeName, nodeCurrentConfig)
			return true
		}
		logger.Infof("Node '%v' has a current config version of '%v'. Waiting for the node's current config version to be '%v'.", nodeName, nodeCurrentConfig, config)
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for node '%v' to have a current config version of '%v'.", nodeName, config)
}

// mcdForNode gets the MCD associated with a node
func mcdForNode(client kubernetes.Interface, node *corev1.Node) (*corev1.Pod, error) {
	// find the MCD pod that has spec.nodeNAME = node.Name and get its name:
	listOptions := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String(),
	}
	listOptions.LabelSelector = labels.SelectorFromSet(labels.Set{"k8s-app": "machine-config-daemon"}).String()

	mcdList, err := client.CoreV1().Pods("openshift-machine-config-operator").List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	if len(mcdList.Items) != 1 {
		if len(mcdList.Items) == 0 {
			return nil, fmt.Errorf("failed to find MCD for node %s", node.Name)
		}
		return nil, fmt.Errorf("too many (%d) MCDs for node %s", len(mcdList.Items), node.Name)
	}
	return &mcdList.Items[0], nil
}

// ExecCmdOnNode finds a node's mcd, and oc rsh's into it to execute a command on the node
// all commands should use /rootfs as root
func ExecCmdOnNode(oc *exutil.CLI, node corev1.Node, subArgs ...string) (*exec.Cmd, error) {
	// Check for an oc binary in $PATH.
	path, err := exec.LookPath("oc")
	if err != nil {
		return nil, fmt.Errorf("could not locate oc command: %w", err)
	}

	mcd, err := mcdForNode(oc.AsAdmin().KubeClient(), &node)
	if err != nil {
		return nil, fmt.Errorf("could not get MCD for node %s: %w", node.Name, err)
	}

	mcdName := mcd.ObjectMeta.Name

	entryPoint := path
	args := []string{"rsh",
		"-n", "openshift-machine-config-operator",
		"-c", "machine-config-daemon",
		mcdName}
	args = append(args, subArgs...)

	cmd := exec.Command(entryPoint, args...)
	return cmd, nil
}

// ExecCmdOnNodeWithError behaves like ExecCmdOnNode, with the exception that
// any errors are returned to the caller for inspection. This allows one to
// execute a command that is expected to fail; e.g., stat /nonexistant/file.
func ExecCmdOnNodeWithError(oc *exutil.CLI, node corev1.Node, subArgs ...string) (string, error) {
	cmd, err := ExecCmdOnNode(oc, node, subArgs...)
	if err != nil {
		return "", err
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// `GetDegradedNode` gets a degraded node from a specified MCP
func GetDegradedNode(oc *exutil.CLI, mcpName string) (corev1.Node, error) {
	// Get nodes in desired pool
	nodes, nodeErr := GetNodesByRole(oc, mcpName)
	if nodeErr != nil {
		return corev1.Node{}, nodeErr
	} else if len(nodes) == 0 {
		return corev1.Node{}, fmt.Errorf("no nodes found in MCP '%v", mcpName)
	}

	// Get degraded node
	for _, node := range nodes {
		if checkMCDState(node, "Degraded") {
			return node, nil
		}
	}

	return corev1.Node{}, errors.New("no degraded node found")
}
