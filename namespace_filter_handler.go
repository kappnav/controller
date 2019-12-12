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
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

func newNamespaceFilter() *namespaceFilter {
	nsFilter := &namespaceFilter{
		permitAllNamespaces: make(map[string]string),
		namespacesForKind:   make(map[string]map[string]string),
	}
	return nsFilter
}

type namespaceFilter struct {
	permitAllNamespaces map[string]string            // kinds for which all namespaces are processed
	namespacesForKind   map[string]map[string]string // map from kind to alloned namespaces
	mutex               sync.Mutex
}

/* Permit all namespaces for a kind. Needs to be called during initialization */
func (nsFilter *namespaceFilter) permitAllNamespacesForKind(kind string) {
	nsFilter.mutex.Lock()
	defer nsFilter.mutex.Unlock()
	if klog.V(3) {
		klog.Infof("permitAllNamesapceForKind called for kind %s", kind)
	}
	nsFilter.permitAllNamespaces[kind] = kind
}

/* Return true if all namespaces are permitted for kind. Note: only the filter is checked.
Whether the resource is not namepsaced is not checked.
*/
func (nsFilter *namespaceFilter) isAllNamespacesPermitted(kind string) bool {
	nsFilter.mutex.Lock()
	defer nsFilter.mutex.Unlock()
	_, ok := nsFilter.permitAllNamespaces[kind]
	return ok
}

/* Add a namespace for a kind. Return true if newly added. Return false if it was previously added */
func (nsFilter *namespaceFilter) addNamespaceForKind(kind string, namespace string) bool {
	nsFilter.mutex.Lock()
	defer nsFilter.mutex.Unlock()

	namespaces, ok := nsFilter.namespacesForKind[kind]
	if !ok {
		/* first namespace to be added */
		namespaces = make(map[string]string)
		nsFilter.namespacesForKind[kind] = namespaces
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

/* permitNamespace adds a new namespace for given kind to be processed for application
   kind: kind for which to add namespace
   namespace: namespace to add for processing the given kind
*/
func (nsFilter *namespaceFilter) permitNamespace(resController *ClusterWatcher, kind string, namespace string) {

	if klog.V(3) {
		klog.Infof("permitNamespace kind: %s, namespace: %s", kind, namespace)
	}

	if !resController.isNamespacePermitted(namespace) {
		// namespace is not in allowed in this kappnav instance
		if klog.V(3) {
			klog.Infof("permitNamespace namespace %s not permitted in this kappnav instance", namespace)
		}
		return
	}

	if !resController.isNamespaced(kind) {
		// resource is not namespaced
		if klog.V(3) {
			klog.Infof("permitNamespaces kind %s not namespaced", kind)
		}
		return
	}

	if nsFilter.isAllNamespacesPermitted(kind) {
		// already permitted for all namespaces
		if klog.V(3) {
			klog.Infof("permitNamespaces kind %s permited for all namespaces", kind)
		}
		return
	}

	if ok := nsFilter.addNamespaceForKind(kind, namespace); ok {
		/* first time adding this namespace. Replay cached objects of this kind matching this namespace */
		if kind == APPLICATION {
			/* applications already has its own handler for all namespaces */
			if klog.V(3) {
				klog.Infof("not replaying applications after adding namespace %s for kind %s", namespace, kind)
			}
			return
		}

		var rw = resController.getResourceWatcher(kind)
		if rw != nil {
			/* we are already watching the resoruce. */
			var resources = resController.listResources(kind)
			for _, resource := range resources {
				key, err := cache.MetaNamespaceKeyFunc(resource)
				if err == nil {
					objNamespace, ok := getNamespace(resource)
					if ok && namespace == objNamespace {
						/* resource in the namespace we want to include */
						data := eventHandlerData{
							funcType: AddFunc,
							kind:     kind,
							key:      key,
							obj:      resource,
							oldObj:   nil,
						}
						if klog.V(3) {
							klog.Infof("replaying %s after adding namespace %s for kind %s", key, namespace, kind)
						}
						batchResourceHandler(resController, rw, &data)
					} else {
						if klog.V(3) {
							klog.Infof("not replaying %s after adding namespace %s for kind %s", key, namespace, kind)
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
		klog.Infof("shouldProcess key: %s, kind: %s", eventData.key, eventData.kind)
	}

	// resource is not namespaced
	if !rw.namespaced {
		if klog.V(3) {
			klog.Infof("shouldProcess true, kind %s not namespaced", eventData.kind)
		}
		return true
	}

	if _, ok := nsFilter.permitAllNamespaces[eventData.kind]; ok && resController.isAllNamespacesPermitted() {
		if klog.V(3) {
			klog.Infof("shouldProcess true, kind: %s is permited for all namespaces", eventData.kind)
		}
		return true
	}

	namespaces, ok := nsFilter.namespacesForKind[eventData.kind]
	if ok {
		/* Allowed namespaces exist. Check if the namespace of the object is allowed. */
		namespace, ok := getNamespace(eventData.obj)
		if ok {
			if _, ok := namespaces[namespace]; ok {
				if klog.V(3) {
					klog.Infof("shouldProcess true, kind: %s is permited for namespace %s", eventData.kind, namespace)
				}
				return true
			}
			if klog.V(3) {
				klog.Infof("shouldProcess false kind: %s is not permited for namespace %s", eventData.kind, namespace)
			}
		}
	} else if klog.V(3) {
		klog.Infof("shouldProcess false kind: %s error getting namespace", eventData.kind)
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
