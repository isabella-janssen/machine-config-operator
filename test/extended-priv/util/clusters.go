package util

import (
	"context"
	"fmt"
	"strings"

	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	// osStreamLabelKey is the MachineSet/CPMS label that identifies the OS image stream
	osStreamLabelKey = "machineconfiguration.openshift.io/osstream"
	// supportedOSStream is the only OS stream value currently supported by the boot image controller
	// Note: This should be updated along with SupportedOSStream in pkg/controller/bootimage/boot_image_controller.go
	supportedOSStream = "rhel-9"
)

// GetClusterVersion returns the cluster version as string value (Ex: 4.8) and cluster build (Ex: 4.8.0-0.nightly-2021-09-28-165247)
func GetClusterVersion(oc *CLI) (string, string, error) {
	clusterBuild, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o", "jsonpath={..desired.version}").Output()
	if err != nil {
		return "", "", err
	}
	splitValues := strings.Split(clusterBuild, ".")
	if len(splitValues) < 2 {
		return "", "", fmt.Errorf("malformed cluster version %q: expected at least major.minor format", clusterBuild)
	}
	clusterVersion := splitValues[0] + "." + splitValues[1]
	return clusterVersion, clusterBuild, nil
}

// GetInfraID returns the infra id
func GetInfraID(oc *CLI) (string, error) {
	infraID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o", "jsonpath='{.status.infrastructureName}'").Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(infraID, "'"), err
}

// IsTechPreviewNoUpgrade checks if the cluster has TechPreviewNoUpgrade feature set enabled
func IsTechPreviewNoUpgrade(oc *CLI) bool {
	featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "could not retrieve feature-gate: %v", err)
	}

	return featureGate.Spec.FeatureSet == configv1.TechPreviewNoUpgrade
}

// IsSingleNodeTopology returns true if the cluster is a SNO cluster
func IsSingleNodeTopology(oc *CLI) bool {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.controlPlaneTopology}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return output == string(configv1.SingleReplicaTopologyMode)
}

// SkipIfUnsupportedOSStreamLabel skips the test if any MachineSet in the cluster carries
// the machineconfiguration.openshift.io/osstream label with a value other than "rhel-9".
// MachineSets that do not carry the label at all are treated as compatible.
func SkipIfUnsupportedOSStreamLabel(oc *CLI) {
	// The label selector matches only MachineSets that have the osstream label set to a value
	// other than the supported one. An empty result means all MachineSets are compatible.
	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"machinesets.machine.openshift.io", "-n", "openshift-machine-api",
		"-l", osStreamLabelKey+","+osStreamLabelKey+"!="+supportedOSStream,
		"-o", "jsonpath={range .items[*]}{.metadata.name}{end}",
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list machinesets")
	if out != "" {
		e2eskipper.Skipf("MachineSet %q has unsupported %s; only %s is supported", out, osStreamLabelKey, supportedOSStream)
	}
}

// SkipIfCPMSHasUnsupportedOSStreamLabel skips the test if the "cluster" ControlPlaneMachineSet
// carries the machineconfiguration.openshift.io/osstream label with a value other than "rhel-9".
// A missing label or a missing CPMS is treated as compatible.
func SkipIfCPMSHasUnsupportedOSStreamLabel(oc *CLI) {
	// The label selector matches only a CPMS that has the osstream label set to an unsupported value.
	// An empty result means that the CPMS is compatible.
	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"controlplanemachinesets.machine.openshift.io", "-n", "openshift-machine-api",
		"-l", osStreamLabelKey+","+osStreamLabelKey+"!="+supportedOSStream,
		"-o", "jsonpath={range .items[*]}{.metadata.name}{end}",
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list machinesets")
	if out != "" {
		e2eskipper.Skipf("ControlPlaneMachineSet %q has unsupported %s; only %s is supported", out, osStreamLabelKey, supportedOSStream)
	}
}

// SkipOnSingleNodeTopology skips the test if the cluster is using single-node topology
func SkipOnSingleNodeTopology(oc *CLI) {
	if IsSingleNodeTopology(oc) {
		e2eskipper.Skipf("This test does not apply to single-node topologies")
	}
}

// SkipIfClusterUnreachable skips the test if the cluster API is unreachable.
// The caller must register this in a BeforeEach BEFORE NewCLI so that the
// skip fires before SetupProject's API calls. Use the var-splitting pattern:
//
//	var oc *CLI
//	g.BeforeEach(func() { SkipIfClusterUnreachable(oc) })
//	oc = NewCLI(...).AsAdmin()
func SkipIfClusterUnreachable(oc *CLI) {
	_, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"infrastructure", "cluster",
		"--request-timeout=10s",
	).Output()
	if err != nil {
		e2eskipper.Skipf("Cluster may be unreachable: %v", err)
	}
}

// SkipIfImageRegistryUnhealthy skips the test if the internal image registry
// is not healthy. The image registry must be available for NewCLI's
// SetupProject to provision dockercfg secrets in new namespaces. On SNO
// clusters the registry shares the single node with all other workloads and
// can remain flaky for extended periods after reboots.
//
// Like SkipIfClusterUnreachable, the caller must register this in a
// BeforeEach BEFORE NewCLI so that the skip fires before SetupProject.
func SkipIfImageRegistryUnhealthy(oc *CLI) {
	admin := oc.AsAdmin().WithoutNamespace()

	replicaStr, err := admin.Run("get").Args(
		"deployment", "image-registry",
		"-n", "openshift-image-registry",
		"-o", "jsonpath={.status.availableReplicas}",
		"--request-timeout=10s",
	).Output()
	if err != nil {
		e2eskipper.Skipf("Cannot check image registry health: %v", err)
		return
	}
	if replicaStr == "" || replicaStr == "0" {
		e2eskipper.Skipf("Image registry has no available replicas — " +
			"namespace setup would fail waiting for dockercfg secrets")
	}

	degraded, err := admin.Run("get").Args(
		"clusteroperator", "image-registry",
		"-o", `jsonpath={.status.conditions[?(@.type=="Degraded")].status}`,
		"--request-timeout=10s",
	).Output()
	if err == nil && degraded == "True" {
		msg, _ := admin.Run("get").Args(
			"clusteroperator", "image-registry",
			"-o", `jsonpath={.status.conditions[?(@.type=="Degraded")].message}`,
			"--request-timeout=10s",
		).Output()
		e2eskipper.Skipf("Image registry ClusterOperator is degraded: %s", msg)
	}

	available, err := admin.Run("get").Args(
		"clusteroperator", "image-registry",
		"-o", `jsonpath={.status.conditions[?(@.type=="Available")].status}`,
		"--request-timeout=10s",
	).Output()
	if err == nil && available != "True" {
		msg, _ := admin.Run("get").Args(
			"clusteroperator", "image-registry",
			"-o", `jsonpath={.status.conditions[?(@.type=="Available")].message}`,
			"--request-timeout=10s",
		).Output()
		e2eskipper.Skipf("Image registry ClusterOperator is not available: %s", msg)
	}

	ocmReplicaStr, err := admin.Run("get").Args(
		"deployment", "controller-manager",
		"-n", "openshift-controller-manager",
		"-o", "jsonpath={.status.availableReplicas}",
		"--request-timeout=10s",
	).Output()
	if err == nil && (ocmReplicaStr == "" || ocmReplicaStr == "0") {
		e2eskipper.Skipf("openshift-controller-manager has no available replicas — " +
			"it provisions dockercfg secrets for new namespaces")
	}
}
