// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	applyconfigurations "github.com/openshift/client-go/config/applyconfigurations"
	clientset "github.com/openshift/client-go/config/clientset/versioned"
	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	fakeconfigv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1/fake"
	configv1alpha1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1alpha1"
	fakeconfigv1alpha1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1alpha1/fake"
	configv1alpha2 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1alpha2"
	fakeconfigv1alpha2 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1alpha2/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
)

// NewSimpleClientset returns a clientset that will respond with the provided objects.
// It's backed by a very simple object tracker that processes creates, updates and deletions as-is,
// without applying any field management, validations and/or defaults. It shouldn't be considered a replacement
// for a real clientset and is mostly useful in simple unit tests.
//
// DEPRECATED: NewClientset replaces this with support for field management, which significantly improves
// server side apply testing. NewClientset is only available when apply configurations are generated (e.g.
// via --with-applyconfig).
func NewSimpleClientset(objects ...runtime.Object) *Clientset {
	o := testing.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &Clientset{tracker: o}
	cs.discovery = &fakediscovery.FakeDiscovery{Fake: &cs.Fake}
	cs.AddReactor("*", "*", testing.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action testing.Action) (handled bool, ret watch.Interface, err error) {
		var opts metav1.ListOptions
		if watchActcion, ok := action.(testing.WatchActionImpl); ok {
			opts = watchActcion.ListOptions
		}
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns, opts)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	return cs
}

// Clientset implements clientset.Interface. Meant to be embedded into a
// struct to get a default implementation. This makes faking out just the method
// you want to test easier.
type Clientset struct {
	testing.Fake
	discovery *fakediscovery.FakeDiscovery
	tracker   testing.ObjectTracker
}

func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

func (c *Clientset) Tracker() testing.ObjectTracker {
	return c.tracker
}

// NewClientset returns a clientset that will respond with the provided objects.
// It's backed by a very simple object tracker that processes creates, updates and deletions as-is,
// without applying any validations and/or defaults. It shouldn't be considered a replacement
// for a real clientset and is mostly useful in simple unit tests.
func NewClientset(objects ...runtime.Object) *Clientset {
	o := testing.NewFieldManagedObjectTracker(
		scheme,
		codecs.UniversalDecoder(),
		applyconfigurations.NewTypeConverter(scheme),
	)
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &Clientset{tracker: o}
	cs.discovery = &fakediscovery.FakeDiscovery{Fake: &cs.Fake}
	cs.AddReactor("*", "*", testing.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action testing.Action) (handled bool, ret watch.Interface, err error) {
		var opts metav1.ListOptions
		if watchActcion, ok := action.(testing.WatchActionImpl); ok {
			opts = watchActcion.ListOptions
		}
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns, opts)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	return cs
}

var (
	_ clientset.Interface = &Clientset{}
	_ testing.FakeClient  = &Clientset{}
)

// ConfigV1 retrieves the ConfigV1Client
func (c *Clientset) ConfigV1() configv1.ConfigV1Interface {
	return &fakeconfigv1.FakeConfigV1{Fake: &c.Fake}
}

// ConfigV1alpha1 retrieves the ConfigV1alpha1Client
func (c *Clientset) ConfigV1alpha1() configv1alpha1.ConfigV1alpha1Interface {
	return &fakeconfigv1alpha1.FakeConfigV1alpha1{Fake: &c.Fake}
}

// ConfigV1alpha2 retrieves the ConfigV1alpha2Client
func (c *Clientset) ConfigV1alpha2() configv1alpha2.ConfigV1alpha2Interface {
	return &fakeconfigv1alpha2.FakeConfigV1alpha2{Fake: &c.Fake}
}
