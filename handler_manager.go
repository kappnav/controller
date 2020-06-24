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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandlersForOneGVR contains event handlers for one GVR
type HandlersForOneGVR struct {
	primaryHandler *resourceActionFunc   // primary handler to be called. Otherwise, defaultPrimaryHandler is called.
	otherHandlers  []*resourceActionFunc // other handlers to be called
}

// HandlerManager contains event handlers for all GVRs
type HandlerManager struct {
	defaultPrimaryHandler *resourceActionFunc
	handlers              map[schema.GroupVersionResource]*HandlersForOneGVR
}

/* Create a new handler manager
 * defaultPrimaryHandler: default primary handler if none is set for a GVR
 */
func newHandlerManager() *HandlerManager {
	ret := &HandlerManager{
		defaultPrimaryHandler: &namespaceFilterHandler,
		handlers:              make(map[schema.GroupVersionResource]*HandlersForOneGVR)}

	ret.setPrimaryHandler(coreApplicationGVR, &batchApplicationHandler)
	ret.setPrimaryHandler(coreCustomResourceDefinitionGVR, &CRDNewHandler)
	ret.setPrimaryHandler(coreKappNavGVR, &KAppNavHandler)
	ret.addOtherHandler(coreDeploymentGVR, &autoCreateAppHandler)
	ret.addOtherHandler(coreStatefulSetGVR, &autoCreateAppHandler)
	ret.addOtherHandler(coreConfigMapGVR, &autoCreateKAMHandler)
	return ret
}

/* Set the primary handler for a GVR
 */
func (mgr *HandlerManager) setPrimaryHandler(gvr schema.GroupVersionResource, primaryHandler *resourceActionFunc) {
	handlersForGVR := mgr.handlers[gvr]
	if handlersForGVR == nil {
		handlersForGVR = &HandlersForOneGVR{
			primaryHandler: primaryHandler,
			otherHandlers:  make([]*resourceActionFunc, 0)}
		mgr.handlers[gvr] = handlersForGVR
	} else {
		handlersForGVR.primaryHandler = primaryHandler
	}
}

/* Add other handlers for a GVR
 */
func (mgr *HandlerManager) addOtherHandler(gvr schema.GroupVersionResource, handler *resourceActionFunc) {
	handlersForGVR := mgr.handlers[gvr]
	if handlersForGVR == nil {
		handlersForGVR = &HandlersForOneGVR{
			primaryHandler: nil,
			otherHandlers:  []*resourceActionFunc{}}
		mgr.handlers[gvr] = handlersForGVR
	}
	handlersForGVR.otherHandlers = append(handlersForGVR.otherHandlers, handler)
}

/* Call all the handlers for a GVR
 */
func (mgr *HandlerManager) callHandlers(gvr schema.GroupVersionResource, resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("callHandlers entry %s %v\n", gvr, eventData))
	}
	handler := mgr.handlers[gvr]
	// TODO: can this be done better? For now, We just accumulate one error and log the rest
	var err error
	if handler != nil {
		if handler.primaryHandler != nil {
			if resController.isEventPermitted(eventData) {
				// Kinds: Application, CustomResourceDefinition, or KAppNav
				// Call batchApplicationHandler, CRDNewHandler, or KAppNavHandler
				err1 := (*handler.primaryHandler)(resController, rw, eventData)
				if err1 != nil {
					err = err1
					if logger.IsEnabled(LogTypeError) {
						logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error calling primary handler for gvr %s, error: %s", gvr, err))
					}
				}
			}
		} else {
			// Kinds: Deployment or StatefulSet
			// Call namespaceFilterHandler > batchResourceHandler
			err2 := (*mgr.defaultPrimaryHandler)(resController, rw, eventData)
			if err2 != nil {
				err = err2
				if logger.IsEnabled(LogTypeError) {
					logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error calling default primary handler for gvr %s, error: %s", gvr, err))
				}
			}
		}
		if resController.isEventPermitted(eventData) {
			// Kinds: Deployment or StatefulSet
			// Call all other handlers (currently only autoCreateAppHandler and autoCreateKAMHandler)
			for _, otherHandler := range handler.otherHandlers {
				err3 := (*otherHandler)(resController, rw, eventData)
				if err3 != nil {
					err = err3
					if logger.IsEnabled(LogTypeError) {
						logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error calling other handler for gvr %s, error: %s", gvr, err))
					}
				}
			}
		}
	} else {
		// Kinds: Application componentKinds that are NOT one of the following:
		//    Application, CustomResourceDefinition, Deployment or StatefulSet
		// Call namespaceFilterHandler > batchResourceHandler
		err4 := (*mgr.defaultPrimaryHandler)(resController, rw, eventData)
		if err4 != nil {
			err = err4
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error calling default primary handler for gvr %s, error: %s", gvr, err))
			}
		}
	}
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("%v\n", err))

	}
	return err
}
