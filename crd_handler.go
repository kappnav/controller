/*
Copyright 2020 IBM Corporation

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
	"fmt"
)

// CRDNewHandler processes changes to Custom Resource Definitions
var CRDNewHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("eventData.funcType: %v resourceWatcher: %v", eventData.funcType, rw))
	}
	key := eventData.key
	obj, exists, err := rw.store.GetByKey(key)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Fetching key %s failed: %v", key, err))
		}
		return err
	}
	if !exists {
		// a CRD has been deleted
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, "A CRD has been deleted")
		}
		resController.deleteGVR(eventData.obj)
	} else {
		// add or modify GVR
		gvr := resController.addGVR(obj)
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Added GVR %s", gvr))
		}
		// add watcher when CRD is added or modified
		if eventData.funcType == AddFunc || eventData.funcType == UpdateFunc {

			if gvr == coreKappNavGVR {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, "KAppNav CRD add or update event")
				}
				resController.AddToWatch(coreKappNavGVR)
			}

			if gvr == coreApplicationGVR {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, "Application CRD add or update event")
				}
				// TODO: need something less hard coded to trigger start watch of deployment when aplication CRD is defined
				resController.AddToWatch(coreApplicationGVR)
				resController.AddToWatch(coreDeploymentGVR)
				resController.AddToWatch(coreStatefulSetGVR)
				resController.AddToWatch(coreConfigMapGVR)
				err = deleteOrphanedAutoCreatedApplications(resController)
				if err != nil {
					if logger.IsEnabled(LogTypeError) {
						logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error deleting orphaned applications: %s", err))
					}
				}
				err = deleteOrphanedAutoCreatedKAMs(resController)
				if err != nil {
					if logger.IsEnabled(LogTypeError) {
						logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error deleting orphaned kams: %s", err))
					}
				}
			}
		}
	}
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, "success")
	}
	return nil
}

// isKAppNavMonitoredResource returns true if the resource is one
// we should always create kappnav status annotations for
func isKAppNavMonitoredResource(resInfo *resourceInfo) bool {
	result := resInfo.gvr == coreApplicationGVR
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("%v %s ", result, resInfo.gvr))
	}
	return result
}
