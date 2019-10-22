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
	if klog.V(2) {
		klog.Infof("CRDNewHandler entry rw: %v\n eventData function: %v", rw, eventData.funcType)
	}
	key := eventData.key
	obj, exists, err := rw.store.GetByKey(key)
	if err != nil {
		klog.Errorf("CRDNewHandler fetching key %s failed: %v", key, err)
		return err
	}
	if !exists {
		// a GVR has been deleted
		klog.Infof("CRDNewHandler a GVR has been deleted, eventData.obj: %v", eventData.obj)
		resController.deleteGVR(eventData.obj)
	} else {
		// add or modify GVR
		gvr := resController.addGVR(obj)
		if eventData.funcType == AddFunc {
			if gvr == coreApplicationGVR {
				if klog.V(4) {
					klog.Infof("CRDNewHandler Application CRD add event")
				}
				// TODO: need something less hard coded to trigger start watch of deployment when aplication CRD is defind
				resController.AddToWatch(coreApplicationGVR)
				resController.AddToWatch(coreDeploymentGVR)
				resController.AddToWatch(coreStatefulSetGVR)
				//resController.AddToWatch(KAppNav)
				err = deleteOrphanedAutoCreatedApplications(resController)
				if err != nil {
					klog.Errorf("Error deleting orphaned applications: %s", err)
				}
			}
		}
	}
	return nil
}
