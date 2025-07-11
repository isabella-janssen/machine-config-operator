// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	configv1 "github.com/openshift/api/config/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// ImagePolicyLister helps list ImagePolicies.
// All objects returned here must be treated as read-only.
type ImagePolicyLister interface {
	// List lists all ImagePolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*configv1.ImagePolicy, err error)
	// ImagePolicies returns an object that can list and get ImagePolicies.
	ImagePolicies(namespace string) ImagePolicyNamespaceLister
	ImagePolicyListerExpansion
}

// imagePolicyLister implements the ImagePolicyLister interface.
type imagePolicyLister struct {
	listers.ResourceIndexer[*configv1.ImagePolicy]
}

// NewImagePolicyLister returns a new ImagePolicyLister.
func NewImagePolicyLister(indexer cache.Indexer) ImagePolicyLister {
	return &imagePolicyLister{listers.New[*configv1.ImagePolicy](indexer, configv1.Resource("imagepolicy"))}
}

// ImagePolicies returns an object that can list and get ImagePolicies.
func (s *imagePolicyLister) ImagePolicies(namespace string) ImagePolicyNamespaceLister {
	return imagePolicyNamespaceLister{listers.NewNamespaced[*configv1.ImagePolicy](s.ResourceIndexer, namespace)}
}

// ImagePolicyNamespaceLister helps list and get ImagePolicies.
// All objects returned here must be treated as read-only.
type ImagePolicyNamespaceLister interface {
	// List lists all ImagePolicies in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*configv1.ImagePolicy, err error)
	// Get retrieves the ImagePolicy from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*configv1.ImagePolicy, error)
	ImagePolicyNamespaceListerExpansion
}

// imagePolicyNamespaceLister implements the ImagePolicyNamespaceLister
// interface.
type imagePolicyNamespaceLister struct {
	listers.ResourceIndexer[*configv1.ImagePolicy]
}
