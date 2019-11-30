/*
Copyright 2019 IBM Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

func newNamespaceFilter() *namespaceFilter {
	nsFilter := &namespaceFilter{
		permitAllNamespaces: make(map[schema.GroupVersionResource]schema.GroupVersionResource),
		namespacesForGVR:    make(map[schema.GroupVersionResource]map[string]string),
	}
	return nsFilter
}

type namespaceFilter struct {
	permitAllNamespaces map[schema.GroupVersionResource]schema.GroupVersionResource // gvrs for which all namespaces are processed
	namespacesForGVR    map[schema.GroupVersionResource]map[string]string           // map from gvr to allowed namespaces
	mutex               sync.Mutex
}

/* Permit all namespaces for a gvr. Needs to be called during initialization */
func (nsFilter *namespaceFilter) permitAllNamespacesForGVR(gvr schema.GroupVersionResource) {
	nsFilter.mutex.Lock()
	defer nsFilter.mutex.Unlock()
	if klog.V(3) {
		klog.Infof("permitAllNamesapceForGVR called for GVR %s", gvr)
	}
	nsFilter.permitAllNamespaces[gvr] = gvr
}

/* Return true if all namespaces are permitted for gvr. Note: only the filter is checked.
Whether the resource is not namepsaced is not checked.
*/
func (nsFilter *namespaceFilter) isAllNamespacesPermitted(gvr schema.GroupVersionResource) bool {
	nsFilter.mutex.Lock()
	defer nsFilter.mutex.Unlock()
	_, ok := nsFilter.permitAllNamespaces[gvr]
	return ok
}

/* Add a namespace for a gvr. Return true if newly added. Return false if it was previously added */
func (nsFilter *namespaceFilter) addNamespaceForGVR(gvr schema.GroupVersionResource, namespace string) bool {
	nsFilter.mutex.Lock()
	defer nsFilter.mutex.Unlock()

	namespaces, ok := nsFilter.namespacesForGVR[gvr]
	if !ok {
		/* first namespace to be added */
		namespaces = make(map[string]string)
		nsFilter.namespacesForGVR[gvr] = namespaces
	}

	_, ok = namespaces[namespace]
	if ok {
		/* already added */
		return false
	}
	/* not already add */
	namespaces[namespace] = namespace
	return true
}

/* permitNamespace adds a new namespace for given GVR to be processed for application
   gvr: GVR for which to add namespace
   namespace: namespace to add for processing the given GVR
*/
func (nsFilter *namespaceFilter) permitNamespace(resController *ClusterWatcher, gvr schema.GroupVersionResource, namespace string) {

	if klog.V(3) {
		klog.Infof("permitNamespace GVR: %s, namespace: %s", gvr, namespace)
	}

	if !resController.isNamespacePermitted(namespace) {
		// namespace is not in allowed in this kappnav instance
		if klog.V(3) {
			klog.Infof("permitNamespace namespace %s not permitted in this kappnav instance", namespace)
		}
		return
	}

	if !resController.isNamespaced(gvr) {
		// resource is not namespaced
		if klog.V(3) {
			klog.Infof("permitNamespaces GVR %s not namespaced", gvr)
		}
		return
	}

	if nsFilter.isAllNamespacesPermitted(gvr) {
		// already permitted for all namespaces
		if klog.V(3) {
			klog.Infof("permitNamespaces GVR %s permited for all namespaces", gvr)
		}
		return
	}

	if ok := nsFilter.addNamespaceForGVR(gvr, namespace); ok {
		/* first time adding this namespace. Replay cached objects of this gvr matching this namespace */
		if gvr == coreApplicationGVR {
			/* applications already has its own handler for all namespaces */
			if klog.V(3) {
				klog.Infof("not replaying applications after adding namespace %s for gvr %s", namespace, gvr)
			}
			return
		}

		var rw = resController.getResourceWatcher(gvr)
		if rw != nil {
			/* we are already watching the resoruce. */
			var resources = resController.listResources(gvr)
			for _, resource := range resources {
				key, err := cache.MetaNamespaceKeyFunc(resource)
				if err == nil {
					objNamespace, ok := getNamespace(resource)
					if ok && namespace == objNamespace {
						/* resource in the namespace we want to include */
						data := eventHandlerData{
							funcType: AddFunc,
							gvr:      gvr,
							key:      key,
							obj:      resource,
							oldObj:   nil,
						}
						if klog.V(3) {
							klog.Infof("replaying %s after adding namespace %s for GVR %s", key, namespace, gvr)
						}
						batchResourceHandler(resController, rw, &data)
					} else {
						if klog.V(3) {
							klog.Infof("not replaying %s after adding namespace %s for GVR %s", key, namespace, gvr)
						}
					}
				}
			}
		}
	}
}

func getNamespace(obj interface{}) (string, bool) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if ok {
		var objMap = unstructuredObj.Object
		metadataObj, ok := objMap[METADATA]
		if ok {
			metadata := metadataObj.(map[string]interface{})
			var ns interface{}
			ns, ok = metadata[NAMESPACE]
			if ok {
				namespace, ok := ns.(string)
				if ok {
					return namespace, true
				}
			}
		}
	}
	return "", false
}

/* shouldProcess returns true if this eventData should be processed */
func (nsFilter *namespaceFilter) shouldProcess(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) bool {
	nsFilter.mutex.Lock()
	defer nsFilter.mutex.Unlock()

	if klog.V(3) {
		klog.Infof("shouldProcess key: %s, gvr: %s", eventData.key, eventData.gvr)
	}

	// resource is not namespaced
	if !rw.namespaced {
		if klog.V(3) {
			klog.Infof("shouldProcess true, gvr %s not namespaced", eventData.gvr)
		}
		return true
	}

	if _, ok := nsFilter.permitAllNamespaces[eventData.gvr]; ok && resController.isAllNamespacesPermitted() {
		if klog.V(3) {
			klog.Infof("shouldProcess true, gvr: %s is permited for all namespaces", eventData.gvr)
		}
		return true
	}

	namespaces, ok := nsFilter.namespacesForGVR[eventData.gvr]
	if ok {
		/* Allowed namespaces exist. Check if the namespace of the object is allowed. */
		namespace, ok := getNamespace(eventData.obj)
		if ok {
			if _, ok := namespaces[namespace]; ok {
				if klog.V(3) {
					klog.Infof("shouldProcess true, gvr: %s is permited for namespace %s", eventData.gvr, namespace)
				}
				return true
			}
			if klog.V(3) {
				klog.Infof("shouldProcess false gvr: %s is not permited for namespace %s", eventData.gvr, namespace)
			}
		}
	} else if klog.V(3) {
		klog.Infof("shouldProcess false no namespaces defined for gvr: %s ", eventData.gvr)
	}
	return false
}

/* Main callback to process resource events */
var namespaceFilterHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	var err error = nil
	if resController.nsFilter.shouldProcess(resController, rw, eventData) {
		err = batchResourceHandler(resController, rw, eventData)
	}
	return err
}
