package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

func (c *LoadBalancerController) syncConfigMap(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CC resource with this namespace/name
	cf, err := c.listers.ConfigMapLister.ConfigMaps(namespace).Get(name)

	//TODO: handle multiple Community Settings in cluster. Now there should be ONLY one configuration per cluster
	if err != nil {
		// The configmap may no longer exist, so we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("ConfigMap '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// retrieve routing rules

	// create a new load balancer if it does not exist

	// sync load balancer backends with the new rules

	c.recorder.Event(cf, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}
