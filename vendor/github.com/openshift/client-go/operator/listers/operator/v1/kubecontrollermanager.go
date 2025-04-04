// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// KubeControllerManagerLister helps list KubeControllerManagers.
// All objects returned here must be treated as read-only.
type KubeControllerManagerLister interface {
	// List lists all KubeControllerManagers in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*operatorv1.KubeControllerManager, err error)
	// Get retrieves the KubeControllerManager from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*operatorv1.KubeControllerManager, error)
	KubeControllerManagerListerExpansion
}

// kubeControllerManagerLister implements the KubeControllerManagerLister interface.
type kubeControllerManagerLister struct {
	listers.ResourceIndexer[*operatorv1.KubeControllerManager]
}

// NewKubeControllerManagerLister returns a new KubeControllerManagerLister.
func NewKubeControllerManagerLister(indexer cache.Indexer) KubeControllerManagerLister {
	return &kubeControllerManagerLister{listers.New[*operatorv1.KubeControllerManager](indexer, operatorv1.Resource("kubecontrollermanager"))}
}
