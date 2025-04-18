package common

const (
	// MCONamespace is the namespace that should be used for all API objects owned by the MCO by default
	MCONamespace = "openshift-machine-config-operator"

	// OpenshiftConfigManagedNamespace is the namespace that has the etc-pki-entitlement/Simple Content Access Cert
	OpenshiftConfigManagedNamespace = "openshift-config-managed"

	// SimpleContentAccessSecret is the secret that holds the SimpleContentAccess cert which provides RHEL entitlements
	SimpleContentAccessSecretName = "etc-pki-entitlement"
	// MachineAPINamespace is the namespace that should be used for all API objects owned by the machine api operator
	MachineAPINamespace = "openshift-machine-api"

	// GlobalPullSecretName is the name of the global pull secret
	GlobalPullSecretName = "pull-secret"

	// OpenshiftConfigNamespace is the namespace that has the global pull secret
	OpenshiftConfigNamespace = "openshift-config"

	// GlobalPullCopySecret is a copy of the cluster wide pull secret. In OCL, this is used if the base image pull secret is not provided.
	GlobalPullSecretCopyName = "global-pull-secret-copy"

	// GeneratedByControllerVersionAnnotationKey is used to tag the machineconfigs generated by the controller with the version of the controller.
	GeneratedByControllerVersionAnnotationKey = "machineconfiguration.openshift.io/generated-by-controller-version"

	// ReleaseImageVersionAnnotationKey is used to tag the rendered machineconfigs & controller config with the release image version.
	ReleaseImageVersionAnnotationKey = "machineconfiguration.openshift.io/release-image-version"

	// OSImageURLOverriddenKey is used to tag a rendered machineconfig when OSImageURL has been overridden from default using machineconfig
	OSImageURLOverriddenKey = "machineconfiguration.openshift.io/os-image-url-overridden"

	// ControllerConfigName is the name of the ControllerConfig object that controllers use
	ControllerConfigName = "machine-config-controller"

	// KernelTypeDefault denominates the default kernel type
	KernelTypeDefault = "default"

	// KernelTypeRealtime denominates the realtime kernel type
	KernelTypeRealtime = "realtime"

	// KernelType64kPages denominates the 64k pages kernel
	KernelType64kPages = "64k-pages"

	// MasterLabel defines the label associated with master node. The master taint uses the same label as taint's key
	MasterLabel = "node-role.kubernetes.io/master"

	// MCNameSuffixAnnotationKey is used to keep track of the machine config name associated with a CR
	MCNameSuffixAnnotationKey = "machineconfiguration.openshift.io/mc-name-suffix"

	// MaxMCNameSuffix is the maximum value of the name suffix of the machine config associated with kubeletconfig and containerruntime objects
	MaxMCNameSuffix int = 9

	// ClusterFeatureInstanceName is a singleton name for featureGate configuration
	ClusterFeatureInstanceName = "cluster"

	// ClusterNodeInstanceName is a singleton name for node configuration
	ClusterNodeInstanceName = "cluster"

	// APIServerInstanceName is a singleton name for APIServer configuration
	APIServerInstanceName = "cluster"

	// APIServerInstanceName is a singleton name for APIServer configuration
	APIServerBootstrapFileLocation = "/etc/mcs/bootstrap/api-server/api-server.yaml"

	// MachineConfigPoolArbiter is the MachineConfigPool name given to the arbiter
	MachineConfigPoolArbiter = "arbiter"

	// MachineConfigPoolMaster is the MachineConfigPool name given to the master
	MachineConfigPoolMaster = "master"

	// MachineConfigPoolWorker is the MachineConfigPool name given to the worker
	MachineConfigPoolWorker = "worker"

	// LayeringEnabledPoolLabel is the label that enables the "layered" workflow path for a pool.
	LayeringEnabledPoolLabel = "machineconfiguration.openshift.io/layering-enabled"

	RebuildPoolLabel = "machineconfiguration.openshift.io/rebuildImage"

	// ExperimentalNewestLayeredImageEquivalentConfigAnnotationKey is the annotation that signifies which rendered config
	// TODO(zzlotnik): Determine if we should use this still.
	ExperimentalNewestLayeredImageEquivalentConfigAnnotationKey = "machineconfiguration.openshift.io/newestImageEquivalentConfig"

	// InternalMCOIgnitionVersion is the ignition version that the MCO converts everything to internally. The intent here is that
	// we should be able to update this constant when we bump the internal ignition version instead of having to hunt down all of
	// the version references and figure out "was this supposed to be explicitly 3.5.0 or just the default version which happens
	// to be 3.5.0 currently". Ideally if you find an explicit "3.5.0", it's supposed to be "3.5.0" version. If it's this constant,
	// it's supposed to be the internal default version.
	InternalMCOIgnitionVersion = "3.5.0"

	// This is the minimum acceptable ignition spec required for boot image updates. This should be updated to be in line with the above.
	MinimumAcceptableStubIgnitionSpec = "3"

	// MachineConfigRoleLabel is the role on MachineConfigs, used to select for pools
	MachineConfigRoleLabel = "machineconfiguration.openshift.io/role"

	// BootImagesConfigMapName is a Configmap of golden bootimages, updated by CVO on an upgrade
	BootImagesConfigMapName = "coreos-bootimages"

	// MCOVersionHashKey is the key for indexing the MCO git version hash stored in the bootimages configmap
	MCOVersionHashKey = "MCOVersionHash"

	// MCOReleaseImageVersionKey is the key for indexing the MCO release version stored in the bootimages configmap
	MCOReleaseImageVersionKey = "MCOReleaseImageVersion"

	// MCOReleaseImageVersionKey is the key for indexing the OCP release version stored in the bootimages configmap
	OCPReleaseVersionKey = "releaseVersion"

	// MCOOperatorKnobsObjectName is the name of the global MachineConfiguration "knobs" object that the MCO watches.
	MCOOperatorKnobsObjectName = "cluster"

	// BootImageOptedInAnnotation is used for book keeping when the MCO applies a default boot image configuration
	BootImageOptedInAnnotation = "machineconfiguration.openshift.io/boot-image-updates-opted-in-at"

	ServiceCARotateAnnotation = "machineconfiguration.openshift.io/service-ca-rotate"

	ServiceCARotateTrue  = "true"
	ServiceCARotateFalse = "false"

	// This is where the installer generated MCS CA bundle was formally stored. This configmap is in the "kube-system" namespace.
	RootCAConfigMapName = "root-ca"

	// This is the name of the configmap bundle and secret where the rotated MCS CA will be stored, in the MCO namespace.
	MachineConfigServerCAName = "machine-config-server-ca"

	// This is the name of the secret which holds the MCS TLS cert. This is generated from the CAs listed above.
	MachineConfigServerTLSSecretName = "machine-config-server-tls"

	// This is the label applied to *-user-data-managed secrets
	MachineConfigServerCAManagedByConfigMapKey = "machineconfiguration.openshift.io/managed-ca-bundle-derived-from-configmap"

	// This is used to key in to the user-data secret
	UserDataKey = "userData"

	// ign* are for the user-data ignition fields
	IgnFieldIgnition = "ignition"
	IgnFieldSource   = "source"
	IgnFieldVersion  = "version"

	// Stub Ignition upgrade related annotation keys
	StubIgnitionVersionAnnotation   = "machineconfiguration.openshift.io/stub-ignition-upgraded-to"
	StubIgnitionTimestampAnnotation = "machineconfiguration.openshift.io/stub-ignition-upgraded-at"
)

// Commonly-used MCO ConfigMap names
const (
	// The name of the machine-config-operator-images ConfigMap.
	MachineConfigOperatorImagesConfigMapName string = "machine-config-operator-images"
	// The name of the machine-config-osimageurl ConfigMap.
	MachineConfigOSImageURLConfigMapName string = "machine-config-osimageurl"
)
