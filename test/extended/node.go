// TODO (MCO-1960): Deduplicate these functions with the helpers defined in /extended-priv/node.go.
package extended

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	o "github.com/onsi/gomega"
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

// execCmdOnNode finds a node's mcd, and oc rsh's into it to execute a command on the node
// all commands should use /rootfs as root
func execCmdOnNode(oc *exutil.CLI, node corev1.Node, subArgs ...string) (*exec.Cmd, error) {
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
	cmd, err := execCmdOnNode(oc, node, subArgs...)
	if err != nil {
		return "", err
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}
