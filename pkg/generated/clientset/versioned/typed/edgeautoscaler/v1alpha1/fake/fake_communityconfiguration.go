// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/lterrac/edge-autoscaler/pkg/apis/edgeautoscaler/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeCommunityConfigurations implements CommunityConfigurationInterface
type FakeCommunityConfigurations struct {
	Fake *FakeEdgeautoscalerV1alpha1
}

var communityconfigurationsResource = schema.GroupVersionResource{Group: "edgeautoscaler.polimi.it", Version: "v1alpha1", Resource: "communityconfigurations"}

var communityconfigurationsKind = schema.GroupVersionKind{Group: "edgeautoscaler.polimi.it", Version: "v1alpha1", Kind: "CommunityConfiguration"}

// Get takes name of the communityConfiguration, and returns the corresponding communityConfiguration object, and an error if there is any.
func (c *FakeCommunityConfigurations) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.CommunityConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(communityconfigurationsResource, name), &v1alpha1.CommunityConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CommunityConfiguration), err
}

// List takes label and field selectors, and returns the list of CommunityConfigurations that match those selectors.
func (c *FakeCommunityConfigurations) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.CommunityConfigurationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(communityconfigurationsResource, communityconfigurationsKind, opts), &v1alpha1.CommunityConfigurationList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.CommunityConfigurationList{ListMeta: obj.(*v1alpha1.CommunityConfigurationList).ListMeta}
	for _, item := range obj.(*v1alpha1.CommunityConfigurationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested communityConfigurations.
func (c *FakeCommunityConfigurations) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(communityconfigurationsResource, opts))
}

// Create takes the representation of a communityConfiguration and creates it.  Returns the server's representation of the communityConfiguration, and an error, if there is any.
func (c *FakeCommunityConfigurations) Create(ctx context.Context, communityConfiguration *v1alpha1.CommunityConfiguration, opts v1.CreateOptions) (result *v1alpha1.CommunityConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(communityconfigurationsResource, communityConfiguration), &v1alpha1.CommunityConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CommunityConfiguration), err
}

// Update takes the representation of a communityConfiguration and updates it. Returns the server's representation of the communityConfiguration, and an error, if there is any.
func (c *FakeCommunityConfigurations) Update(ctx context.Context, communityConfiguration *v1alpha1.CommunityConfiguration, opts v1.UpdateOptions) (result *v1alpha1.CommunityConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(communityconfigurationsResource, communityConfiguration), &v1alpha1.CommunityConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CommunityConfiguration), err
}

// Delete takes name of the communityConfiguration and deletes it. Returns an error if one occurs.
func (c *FakeCommunityConfigurations) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(communityconfigurationsResource, name), &v1alpha1.CommunityConfiguration{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCommunityConfigurations) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(communityconfigurationsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.CommunityConfigurationList{})
	return err
}

// Patch applies the patch and returns the patched communityConfiguration.
func (c *FakeCommunityConfigurations) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.CommunityConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(communityconfigurationsResource, name, pt, data, subresources...), &v1alpha1.CommunityConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CommunityConfiguration), err
}