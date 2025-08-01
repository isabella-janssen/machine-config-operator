// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	context "context"
	time "time"

	apimachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	versioned "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	internalinterfaces "github.com/openshift/client-go/machineconfiguration/informers/externalversions/internalinterfaces"
	machineconfigurationv1 "github.com/openshift/client-go/machineconfiguration/listers/machineconfiguration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// MachineOSConfigInformer provides access to a shared informer and lister for
// MachineOSConfigs.
type MachineOSConfigInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() machineconfigurationv1.MachineOSConfigLister
}

type machineOSConfigInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewMachineOSConfigInformer constructs a new informer for MachineOSConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewMachineOSConfigInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredMachineOSConfigInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredMachineOSConfigInformer constructs a new informer for MachineOSConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredMachineOSConfigInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MachineconfigurationV1().MachineOSConfigs().List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MachineconfigurationV1().MachineOSConfigs().Watch(context.Background(), options)
			},
			ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MachineconfigurationV1().MachineOSConfigs().List(ctx, options)
			},
			WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MachineconfigurationV1().MachineOSConfigs().Watch(ctx, options)
			},
		},
		&apimachineconfigurationv1.MachineOSConfig{},
		resyncPeriod,
		indexers,
	)
}

func (f *machineOSConfigInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredMachineOSConfigInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *machineOSConfigInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&apimachineconfigurationv1.MachineOSConfig{}, f.defaultInformer)
}

func (f *machineOSConfigInformer) Lister() machineconfigurationv1.MachineOSConfigLister {
	return machineconfigurationv1.NewMachineOSConfigLister(f.Informer().GetIndexer())
}
