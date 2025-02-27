// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"
)

// ClusterVersionOperatorLister helps list ClusterVersionOperators.
// All objects returned here must be treated as read-only.
type ClusterVersionOperatorLister interface {
	// List lists all ClusterVersionOperators in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ClusterVersionOperator, err error)
	// Get retrieves the ClusterVersionOperator from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ClusterVersionOperator, error)
	ClusterVersionOperatorListerExpansion
}

// clusterVersionOperatorLister implements the ClusterVersionOperatorLister interface.
type clusterVersionOperatorLister struct {
	listers.ResourceIndexer[*v1alpha1.ClusterVersionOperator]
}

// NewClusterVersionOperatorLister returns a new ClusterVersionOperatorLister.
func NewClusterVersionOperatorLister(indexer cache.Indexer) ClusterVersionOperatorLister {
	return &clusterVersionOperatorLister{listers.New[*v1alpha1.ClusterVersionOperator](indexer, v1alpha1.Resource("clusterversionoperator"))}
}
