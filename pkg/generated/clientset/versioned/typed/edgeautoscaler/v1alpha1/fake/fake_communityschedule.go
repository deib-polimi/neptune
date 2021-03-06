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

// FakeCommunitySchedules implements CommunityScheduleInterface
type FakeCommunitySchedules struct {
	Fake *FakeEdgeautoscalerV1alpha1
	ns   string
}

var communityschedulesResource = schema.GroupVersionResource{Group: "edgeautoscaler.polimi.it", Version: "v1alpha1", Resource: "communityschedules"}

var communityschedulesKind = schema.GroupVersionKind{Group: "edgeautoscaler.polimi.it", Version: "v1alpha1", Kind: "CommunitySchedule"}

// Get takes name of the communitySchedule, and returns the corresponding communitySchedule object, and an error if there is any.
func (c *FakeCommunitySchedules) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.CommunitySchedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(communityschedulesResource, c.ns, name), &v1alpha1.CommunitySchedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CommunitySchedule), err
}

// List takes label and field selectors, and returns the list of CommunitySchedules that match those selectors.
func (c *FakeCommunitySchedules) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.CommunityScheduleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(communityschedulesResource, communityschedulesKind, c.ns, opts), &v1alpha1.CommunityScheduleList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.CommunityScheduleList{ListMeta: obj.(*v1alpha1.CommunityScheduleList).ListMeta}
	for _, item := range obj.(*v1alpha1.CommunityScheduleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested communitySchedules.
func (c *FakeCommunitySchedules) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(communityschedulesResource, c.ns, opts))

}

// Create takes the representation of a communitySchedule and creates it.  Returns the server's representation of the communitySchedule, and an error, if there is any.
func (c *FakeCommunitySchedules) Create(ctx context.Context, communitySchedule *v1alpha1.CommunitySchedule, opts v1.CreateOptions) (result *v1alpha1.CommunitySchedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(communityschedulesResource, c.ns, communitySchedule), &v1alpha1.CommunitySchedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CommunitySchedule), err
}

// Update takes the representation of a communitySchedule and updates it. Returns the server's representation of the communitySchedule, and an error, if there is any.
func (c *FakeCommunitySchedules) Update(ctx context.Context, communitySchedule *v1alpha1.CommunitySchedule, opts v1.UpdateOptions) (result *v1alpha1.CommunitySchedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(communityschedulesResource, c.ns, communitySchedule), &v1alpha1.CommunitySchedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CommunitySchedule), err
}

// Delete takes name of the communitySchedule and deletes it. Returns an error if one occurs.
func (c *FakeCommunitySchedules) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(communityschedulesResource, c.ns, name), &v1alpha1.CommunitySchedule{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCommunitySchedules) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(communityschedulesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.CommunityScheduleList{})
	return err
}

// Patch applies the patch and returns the patched communitySchedule.
func (c *FakeCommunitySchedules) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.CommunitySchedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(communityschedulesResource, c.ns, name, pt, data, subresources...), &v1alpha1.CommunitySchedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CommunitySchedule), err
}
