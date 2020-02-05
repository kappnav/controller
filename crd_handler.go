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
	"k8s.io/klog"
)

const (
	// LibertyApp kind
	LibertyApp = "Liberty-App"
	// LibertyCollective kind
	LibertyCollective = "Liberty-Collective"
	// WasNdCell kind
	WasNdCell = "WAS-ND-Cell"
	// WasTraditionalApp kind
	WasTraditionalApp = "WAS-Traditional-App"
)

var (
	coreLibertyAppGVR = schema.GroupVersionResource{
		Group:    "kappnav.io",
		Version:  "v1beta1",
		Resource: "liberty-apps",
	}
	coreLibertyCollectiveGVR = schema.GroupVersionResource{
		Group:    "kappnav.io",
		Version:  "v1beta1",
		Resource: "liberty-collectives",
	}
	coreWasTraditionalAppGVR = schema.GroupVersionResource{
		Group:    "kappnav.io",
		Version:  "v1beta1",
		Resource: "was-traditional-apps",
	}
	coreWasNDCellGVR = schema.GroupVersionResource{
		Group:    "kappnav.io",
		Version:  "v1beta1",
		Resource: "was-nd-cells",
	}
)

func init() {
	coreKindToGVR["WAS-Traditional-App"] = coreWasTraditionalAppGVR
	coreKindToGVR["WAS-ND-Cell"] = coreWasNDCellGVR
	coreKindToGVR["Liberty-Collective"] = coreLibertyCollectiveGVR
	coreKindToGVR["Liberty-App"] = coreLibertyAppGVR
}

// CRDNewHandler processes changes to Custom Resource Definitions
var CRDNewHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	if klog.V(2) {
		klog.Infof("CRDNewHandler entry eventData.funcType: %v resourceWatcher: %v", eventData.funcType, rw)
	}
	key := eventData.key
	obj, exists, err := rw.store.GetByKey(key)
	if err != nil {
		klog.Errorf("CRDNewHandler fetching key %s failed: %v", key, err)
		return err
	}
	if !exists {
		// a CRD has been deleted
		if klog.V(4) {
			klog.Infof("CRDNewHandler a CRD has been deleted")
		}
		resController.deleteGVR(eventData.obj)
	} else {
		// add or modify GVR
		gvr := resController.addGVR(obj)
		if klog.V(4) {
			klog.Infof("CRDNewHandler added GVR %s", gvr)
		}
		if eventData.funcType == AddFunc {
			if gvr == coreApplicationGVR {
				if klog.V(4) {
					klog.Infof("CRDNewHandler Application CRD add event")
				}
				// TODO: need something less hard coded to trigger start watch of deployment when aplication CRD is defind
				resController.AddToWatch(coreApplicationGVR)
				resController.AddToWatch(coreDeploymentGVR)
				resController.AddToWatch(coreStatefulSetGVR)
				resController.AddToWatch(coreDeploymentConfigGVR)
				//resController.AddToWatch(KAppNav)
				err = deleteOrphanedAutoCreatedApplications(resController)
				if err != nil {
					klog.Errorf("Error deleting orphaned applications: %s", err)
				}
			} else if (gvr == coreWasTraditionalAppGVR) ||
				(gvr == coreWasNDCellGVR) ||
				(gvr == coreLibertyAppGVR) ||
				(gvr == coreLibertyCollectiveGVR) {
				if resController.isAllNamespacesPermitted() {
					// always generate status for WAS related kinds
					resController.nsFilter.permitAllNamespacesForGVR(gvr)
				} else {
					// only get status for WAS kinds in namespaces allowed in this kappnav instance
					for _, ns := range resController.namespaces {
						resController.nsFilter.permitNamespace(resController, gvr, ns)
					}
				}
				resController.AddToWatch(gvr)
			}
		}
	}
	if klog.V(4) {
		klog.Infof("CRDNewHandler exit success")
	}
	return nil
}
