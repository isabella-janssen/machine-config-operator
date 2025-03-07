// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

import (
	machineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

// MachineOSBuildStatusApplyConfiguration represents a declarative configuration of the MachineOSBuildStatus type for use
// with apply.
type MachineOSBuildStatusApplyConfiguration struct {
	Conditions            []metav1.ConditionApplyConfiguration         `json:"conditions,omitempty"`
	Builder               *MachineOSBuilderReferenceApplyConfiguration `json:"builder,omitempty"`
	RelatedObjects        []ObjectReferenceApplyConfiguration          `json:"relatedObjects,omitempty"`
	BuildStart            *apismetav1.Time                             `json:"buildStart,omitempty"`
	BuildEnd              *apismetav1.Time                             `json:"buildEnd,omitempty"`
	DigestedImagePushSpec *machineconfigurationv1.ImageDigestFormat    `json:"digestedImagePushSpec,omitempty"`
}

// MachineOSBuildStatusApplyConfiguration constructs a declarative configuration of the MachineOSBuildStatus type for use with
// apply.
func MachineOSBuildStatus() *MachineOSBuildStatusApplyConfiguration {
	return &MachineOSBuildStatusApplyConfiguration{}
}

// WithConditions adds the given value to the Conditions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Conditions field.
func (b *MachineOSBuildStatusApplyConfiguration) WithConditions(values ...*metav1.ConditionApplyConfiguration) *MachineOSBuildStatusApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithConditions")
		}
		b.Conditions = append(b.Conditions, *values[i])
	}
	return b
}

// WithBuilder sets the Builder field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Builder field is set to the value of the last call.
func (b *MachineOSBuildStatusApplyConfiguration) WithBuilder(value *MachineOSBuilderReferenceApplyConfiguration) *MachineOSBuildStatusApplyConfiguration {
	b.Builder = value
	return b
}

// WithRelatedObjects adds the given value to the RelatedObjects field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the RelatedObjects field.
func (b *MachineOSBuildStatusApplyConfiguration) WithRelatedObjects(values ...*ObjectReferenceApplyConfiguration) *MachineOSBuildStatusApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithRelatedObjects")
		}
		b.RelatedObjects = append(b.RelatedObjects, *values[i])
	}
	return b
}

// WithBuildStart sets the BuildStart field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the BuildStart field is set to the value of the last call.
func (b *MachineOSBuildStatusApplyConfiguration) WithBuildStart(value apismetav1.Time) *MachineOSBuildStatusApplyConfiguration {
	b.BuildStart = &value
	return b
}

// WithBuildEnd sets the BuildEnd field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the BuildEnd field is set to the value of the last call.
func (b *MachineOSBuildStatusApplyConfiguration) WithBuildEnd(value apismetav1.Time) *MachineOSBuildStatusApplyConfiguration {
	b.BuildEnd = &value
	return b
}

// WithDigestedImagePushSpec sets the DigestedImagePushSpec field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DigestedImagePushSpec field is set to the value of the last call.
func (b *MachineOSBuildStatusApplyConfiguration) WithDigestedImagePushSpec(value machineconfigurationv1.ImageDigestFormat) *MachineOSBuildStatusApplyConfiguration {
	b.DigestedImagePushSpec = &value
	return b
}
