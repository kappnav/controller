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
	"k8s.io/klog"
)

// CRDNewHandler processes changes to Custom Resource Definitions
var CRDNewHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	key := eventData.key
	obj, exists, err := rw.store.GetByKey(key)
	if err != nil {
		klog.Errorf("fetching key %s failed: %v", key, err)
		return err
	}
	if !exists {
		// a kind has been deleted
		resController.deleteKind(eventData.obj)
	} else {
		// add or modify kind
		kind := resController.addKind(obj)
		if eventData.funcType == AddFunc {
			if kind == APPLICATION {
				// TODO: need something less hard coded to trigger start watch of deployment when aplication CRD is defind
				resController.AddToWatch(APPLICATION)
				resController.AddToWatch(DEPLOYMENT)
				resController.AddToWatch(STATEFULSET)
				err = deleteOrphanedAutoCreatedApplications(resController)
				if err != nil {
					klog.Errorf("Error deleting orphaned applications: %s", err)
				}
			} else if (kind == WasTraditionalApp) ||
				(kind == WasNdCell) ||
				(kind == LibertyApp) ||
				(kind == LibertyCollective) {
				if resController.isAllNamespacesPermitted() {
					// always generate status for WAS related kinds
					resController.nsFilter.permitAllNamespacesForKind(kind)
				} else {
					// only get status for WAS kinds in namespaces allowed in this kappnav instance
					for _, ns := range resController.namespaces {
						resController.nsFilter.permitNamespace(resController, kind, ns)
					}
				}
				resController.AddToWatch(kind)
			}
		}
	}
	return nil
}
