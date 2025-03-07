// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

import (
	corev1 "k8s.io/api/core/v1"
)

// ImageChangeTriggerApplyConfiguration represents a declarative configuration of the ImageChangeTrigger type for use
// with apply.
type ImageChangeTriggerApplyConfiguration struct {
	LastTriggeredImageID *string                 `json:"lastTriggeredImageID,omitempty"`
	From                 *corev1.ObjectReference `json:"from,omitempty"`
	Paused               *bool                   `json:"paused,omitempty"`
}

// ImageChangeTriggerApplyConfiguration constructs a declarative configuration of the ImageChangeTrigger type for use with
// apply.
func ImageChangeTrigger() *ImageChangeTriggerApplyConfiguration {
	return &ImageChangeTriggerApplyConfiguration{}
}

// WithLastTriggeredImageID sets the LastTriggeredImageID field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the LastTriggeredImageID field is set to the value of the last call.
func (b *ImageChangeTriggerApplyConfiguration) WithLastTriggeredImageID(value string) *ImageChangeTriggerApplyConfiguration {
	b.LastTriggeredImageID = &value
	return b
}

// WithFrom sets the From field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the From field is set to the value of the last call.
func (b *ImageChangeTriggerApplyConfiguration) WithFrom(value corev1.ObjectReference) *ImageChangeTriggerApplyConfiguration {
	b.From = &value
	return b
}

// WithPaused sets the Paused field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Paused field is set to the value of the last call.
func (b *ImageChangeTriggerApplyConfiguration) WithPaused(value bool) *ImageChangeTriggerApplyConfiguration {
	b.Paused = &value
	return b
}
