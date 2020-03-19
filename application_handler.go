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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// Return true if labels1 and labels2 are the same
func sameLabels(labels1 map[string]string, labels2 map[string]string) bool {
	if labels1 == nil && labels2 == nil {
		return true
	}
	if labels1 == nil || labels2 == nil {
		return false
	}
	if len(labels1) != len(labels2) {
		return false
	}

	// both map have the same length
	for key, val1 := range labels1 {
		var val2 string
		var ok bool
		if val2, ok = labels2[key]; !ok {
			// key does not exist in labels2
			return false
		}
		if val1 != val2 {
			// value not the same
			return false
		}
	}
	// all keys in labels1 are also in labels2, and map to the same value
	return true
}

// Return true if the labels defined in matchLabels also are defined in labels
// matchLabels: match labels defined in the application
// labels: labels in the resource
// Return false if matchLabels is nil or empty
func labelsMatch(matchLabels map[string]string, labels map[string]string) bool {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("matchLabels %s, labels: %s\n", matchLabels, labels))
	}
	if matchLabels == nil || len(matchLabels) == 0 {
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, "false\n ")
		}
		return false
	}
	for key, val := range matchLabels {
		otherVal, ok := labels[key]
		if !ok {
			if logger.IsEnabled(LogTypeExit) {
				logger.Log(CallerName(), LogTypeExit, "false\n ")
			}
			return false
		}
		if strings.Compare(val, otherVal) != 0 {
			if logger.IsEnabled(LogTypeExit) {
				logger.Log(CallerName(), LogTypeExit, "false\n ")
			}
			return false
		}
	}
	// everything match
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, "true\n ")
	}
	return true
}

// Return true if input group and kind are contained in array of groupKind
func isContainedIn(arr []groupKind, resInfo *resourceInfo) bool {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("resInfo.kind: %s resInfo.apiVersion %s groupKinds: %v\n", resInfo.kind, resInfo.apiVersion, arr))
	}
	group := getGroupFromAPIVersion(resInfo.apiVersion)
	for _, gk := range arr {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("gk.kind: %s gk.group %s\n", gk.kind, gk.group))
		}
		if strings.Compare(gk.kind, resInfo.kind) == 0 &&
			strings.Compare(gk.group, group) == 0 {
			if logger.IsEnabled(LogTypeExit) {
				logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("exit true resInfo.kind: %s resInfo.apiVersion %s groupKinds: %v\n", resInfo.kind, resInfo.apiVersion, arr))
			}
			return true
		}
	}

	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("exit false resInfo.kind: %s resInfo.apiVersion %s groupKinds: %v\n", resInfo.kind, resInfo.apiVersion, arr))
	}
	return false
}

func getGroupFromAPIVersion(apiVersion string) string {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("apiVersion: %s\n", apiVersion))
	}
	var group string
	if strings.Contains(apiVersion, "/") {
		split := strings.Split(apiVersion, "/")
		group = split[0]
	}

	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("apiVersion: %s group: %s\n", apiVersion, group))
	}
	return group
}

// return true if the input string is contaied in array of strings
func isContainedInStringArray(arr []string, inStr string) bool {
	for _, str := range arr {
		if strings.Compare(str, inStr) == 0 {
			return true
		}
	}
	return false
}

// Return true if labels match the given expressions
// Return false if expressions is nil or empty
func expressionsMatch(expressions []matchExpression, labels map[string]string) bool {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("expressions: %s len:%d, labels: %s\n", expressions, len(expressions), labels))
	}
	if expressions == nil || len(expressions) == 0 {
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, "nil or empty expressions")
		}
		return false
	}
	for _, expr := range expressions {
		value, ok := labels[expr.key]
		switch expr.operator {
		case OperatorIn:
			if !ok || !isContainedInStringArray(expr.values, value) {
				// not in
				if logger.IsEnabled(LogTypeExit) {
					logger.Log(CallerName(), LogTypeExit, "false\n")
				}
				return false
			}
		case OperatorNotIn:
			if !ok || isContainedInStringArray(expr.values, value) {
				// label deos notexists or there is a match
				if logger.IsEnabled(LogTypeExit) {
					logger.Log(CallerName(), LogTypeExit, "false\n")
				}
				return false
			}
		case OperatorExists:
			if !ok {
				// does not exist
				if logger.IsEnabled(LogTypeExit) {
					logger.Log(CallerName(), LogTypeExit, "false\n")
				}
				return false
			}
		case OperatorDoesNotExist:
			if ok {
				// exists
				if logger.IsEnabled(LogTypeExit) {
					logger.Log(CallerName(), LogTypeExit, "false\n")
				}
				return false
			}
		default:
			if logger.IsEnabled(LogTypeExit) {
				logger.Log(CallerName(), LogTypeExit, "false\n")
			}
			return false
		}
	}

	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, "true\n")
	}
	return true
}

/* Check if resource namespace matches what application requires of its components.
Return true if resource is not namespace, or
      resource namespace matches application namespace, or
      resource namespace is in the list of application's component namespaces
*/
func resourceNamespaceMatchesApplicationComponentNamespaces(resController *ClusterWatcher, appResInfo *appResourceInfo, namespace string) bool {

	if namespace == "" {
		// resource not namespaced
		return true
	}

	if appResInfo.namespace == namespace && resController.isNamespacePermitted(namespace) {
		// same namespace
		return true
	}
	// Different namespace. Check if this Application allows this namespace
	_, ok := appResInfo.componentNamespaces[namespace]
	return ok
}

// Return true if the resource is a component of the application
func isComponentOfApplication(resController *ClusterWatcher, appResInfo *appResourceInfo, resInfo *resourceInfo) bool {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("app: %s, resource: %s\n", appResInfo.name, resInfo.name))
	}

	if !resourceNamespaceMatchesApplicationComponentNamespaces(resController, appResInfo, resInfo.namespace) {
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("false due to namespace: resource is %s/%s, application is %s/%s, component namespaces is: %s", resInfo.namespace, resInfo.name, appResInfo.namespace, appResInfo.name, appResInfo.componentNamespaces))
		}
		return false
	}

	if isSameResource(&appResInfo.resourceInfo, resInfo) {
		// self
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("false: resource is self\n"))
		}
		return false
	}
	if !isContainedIn(appResInfo.componentKinds, resInfo) {
		// resource kind not what the application wants to include
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("false: component kinds: %v, resource apiVersion: %s kind: %s \n", appResInfo.componentKinds, resInfo.apiVersion, resInfo.kind))
		}
		return false
	}
	var hasMatchLabels = true
	if len(appResInfo.matchLabels) == 0 {
		hasMatchLabels = false
	}
	var hasMatchExpressions = true
	if len(appResInfo.matchExpressions) == 0 {
		hasMatchExpressions = false
	}

	var ret bool
	if hasMatchLabels && hasMatchExpressions {
		ret = labelsMatch(appResInfo.matchLabels, resInfo.labels) &&
			expressionsMatch(appResInfo.matchExpressions, resInfo.labels)
	} else if hasMatchLabels {
		ret = labelsMatch(appResInfo.matchLabels, resInfo.labels)
	} else if hasMatchExpressions {
		ret = expressionsMatch(appResInfo.matchExpressions, resInfo.labels)
	} else {
		ret = false
	}

	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("%t\n", ret))
	}
	return ret
}

// Delete given resource from Kube
func deleteResource(resController *ClusterWatcher, resInfo *resourceInfo) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("GVR: %s namespace: %s name: %s\n", resInfo.gvr, resInfo.namespace, resInfo.name))
	}
	gvr, ok := resController.getWatchGVR(resInfo.gvr)
	if ok {
		// resource still being watched
		var intfNoNS = resController.plugin.dynamicClient.Resource(gvr)
		var intf dynamic.ResourceInterface
		if resInfo.namespace != "" {
			intf = intfNoNS.Namespace(resInfo.namespace)
		} else {
			intf = intfNoNS
		}

		var err error
		err = intf.Delete(resInfo.name, nil)
		if err != nil {
			if logger.IsEnabled(LogTypeExit) {
				logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("error: %s %s %s %s\n", resInfo.gvr, resInfo.namespace, resInfo.name, err))
			}
			return err
		}
	}

	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("success: %s %s %s\n", resInfo.gvr, resInfo.namespace, resInfo.name))
	}
	return nil
}

// Check if resource is deleted
func resourceDeleted(resController *ClusterWatcher, resInfo *resourceInfo) (bool, error) {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf(" %s %s %s\n", resInfo.gvr, resInfo.namespace, resInfo.name))
	}

	gvr, ok := resController.getWatchGVR(resInfo.gvr)
	if ok {
		var intfNoNS = resController.plugin.dynamicClient.Resource(gvr)
		var intf dynamic.ResourceInterface
		if resInfo.namespace != "" {
			intf = intfNoNS.Namespace(resInfo.namespace)
		} else {
			intf = intfNoNS
		}

		// fetch the current resource
		var err error
		_, err = intf.Get(resInfo.name, metav1.GetOptions{})
		if err == nil {
			return false, fmt.Errorf("Resource %s %s %s not deleted", resInfo.gvr, resInfo.namespace, resInfo.name)
		}
		// TODO: better checking between error and resource deleted
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("    true: %s %s %s %s\n", resInfo.gvr, resInfo.namespace, resInfo.name, err))
		}
		return true, nil
	}
	return true, nil
}

// Return applications for which a resource is a direct sub-component
func getApplicationsForResource(resController *ClusterWatcher, resInfo *resourceInfo) []*appResourceInfo {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("%s\n", resInfo.name))
	}
	var ret = make([]*appResourceInfo, 0)
	// loop over all applications
	var apps = resController.listResources(coreApplicationGVR)
	for _, app := range apps {
		var unstructuredObj = app.(*unstructured.Unstructured)
		var appResInfo = &appResourceInfo{}
		if err := resController.parseAppResource(unstructuredObj, appResInfo); err == nil {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    checking application: %s\n", appResInfo.name))
			}
			if isComponentOfApplication(resController, appResInfo, resInfo) {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    found application: %s\n", appResInfo.name))
				}
				ret = append(ret, appResInfo)
			}
		} else {
			// shouldn't happen
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Unable to parse application resource %s\n", err))
			}
		}
	}
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("%v", ret))
	}
	return ret
}

/* Recursive find all applications and ancestors for a resource
   alreadyFound: map of applications that have already been processed
*/
func findAllApplicationsForResource(resController *ClusterWatcher, obj interface{}, alreadyFound map[string]*resourceInfo) {

	var unstructuredObj = obj.(*unstructured.Unstructured)
	var resInfo = &resourceInfo{}
	resController.parseResource(unstructuredObj, resInfo)

	findAllApplicationsForResourceHelper(resController, resInfo, alreadyFound)
	return
}

func findAllApplicationsForResourceHelper(resController *ClusterWatcher, resInfo *resourceInfo, alreadyFound map[string]*resourceInfo) {

	if resInfo.gvr == coreApplicationGVR {
		key := resInfo.key()
		_, exists := alreadyFound[key]
		if exists {
			return
		}
		alreadyFound[key] = resInfo
	}

	// recursively find all parent applications
	for _, appResInfo := range getApplicationsForResource(resController, resInfo) {
		findAllApplicationsForResourceHelper(resController, &appResInfo.resourceInfo, alreadyFound)
	}
}

// Callback to handle resource changes
// TODO: DO not add resource if only kappnav status changed
var batchResourceHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	key := eventData.key
	obj, exists, err := rw.store.GetByKey(key)
	applications := make(map[string]*resourceInfo)
	nonApplications := make(map[string]*resourceInfo)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("fetching key %s from store failed: %v", key, err))
		}
		return err
	}
	if !exists {
		// delete resource
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    processing deleted resource %s\n", key))
		}
		// batch up all parent applications
		findAllApplicationsForResource(resController, eventData.obj, applications)
	} else {
		var resInfo = &resourceInfo{}
		resController.parseResource(eventData.obj.(*unstructured.Unstructured), resInfo)
		if eventData.funcType == UpdateFunc {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    processig updated resource : %s\n", key))
			}
			var oldResInfo = &resourceInfo{}
			resController.parseResource(eventData.oldObj.(*unstructured.Unstructured), oldResInfo)
			if !sameLabels(oldResInfo.labels, resInfo.labels) {
				// label changed. Update ancestors matched by old labels
				findAllApplicationsForResource(resController, eventData.oldObj, applications)
			}
		} else {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("   processing added resource: %s\n", key))
			}
		}
		// find all ancestors
		findAllApplicationsForResource(resController, obj, applications)

		nonApplications[resInfo.key()] = resInfo

	}
	resourceToBatch := batchResources{
		applications:    applications,
		nonApplications: nonApplications,
	}
	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    Sending %d applications and %d resources on channel\n", len(resourceToBatch.applications), len(resourceToBatch.nonApplications)))
	}
	resController.resourceChannel.send(&resourceToBatch)
	return nil
}

// Start watching component kinds of the application. Also put
// application on batch of applications to recalculate status
func startWatchApplicationComponentKinds(resController *ClusterWatcher, obj interface{}, applications map[string]*resourceInfo) error {
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%T %s\n", obj, obj))
	}
	switch obj.(type) {
	case *unstructured.Unstructured:
		var unstructuredObj = obj.(*unstructured.
			Unstructured)

		var appInfo = &appResourceInfo{}
		if err := resController.parseAppResource(unstructuredObj, appInfo); err == nil {
			// start watching all component kinds of the application
			var componentKinds = appInfo.componentKinds
			nsFilter := resController.nsFilter
			for _, elem := range componentKinds {
				// watch every installed version of the group
				for _, gvr := range elem.gvrSet {
					/* Start processing kinds in the application's namespace */
					nsFilter.permitNamespace(resController, gvr, appInfo.resourceInfo.namespace)

					/* also permit namespaces in the kappnav.component.namespaces annotation */
					for _, ns := range appInfo.componentNamespaces {
						nsFilter.permitNamespace(resController, gvr, ns)
					}

					err := resController.AddToWatch(gvr)
					if err != nil {
						// TODO: should we continue to process the rest of kinds?
						return err
					}
				}
			}
			applications[appInfo.resourceInfo.key()] = &appInfo.resourceInfo
		}

		return nil

	default:
		return fmt.Errorf("    batchAddModifyApplication.addApplication: not Unstructured: type: %T val: % s", obj, obj)
	}
}

// Handle application changes
// TODO: Do not add applications to be processed if only kappnav status changed
var batchApplicationHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, "\n")
	}

	key := eventData.key
	obj, exists, err := rw.store.GetByKey(key)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("   fetching key %s failed: %v", key, err))
		}
		return err
	}
	applications := make(map[string]*resourceInfo)
	nonApplications := make(map[string]*resourceInfo)
	if !exists {
		// application is gone. Update parent applications
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    processing application deleted: %s\n", key))
		}
		// batch up all ancestor applications
		findAllApplicationsForResource(resController, eventData.obj, applications)
	} else {
		if eventData.funcType == UpdateFunc {
			// application updated
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    processing application updated: %s\n", key))
			}

			var oldResInfo = &resourceInfo{}
			resController.parseResource(eventData.oldObj.(*unstructured.Unstructured), oldResInfo)
			var newResInfo = &resourceInfo{}
			resController.parseResource(eventData.obj.(*unstructured.Unstructured), newResInfo)
			// Something changed. batch up ancestors of application
			// TODO: optimize by finding ancestors only if label or
			// selector changed.  Note that a label change affects which
			// parent applications selects this application. A selector
			// changes affects which sub-components are included in calculation
			findAllApplicationsForResource(resController, eventData.oldObj, applications)
		} else {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    processing application added: %s\n", key))
			}
		}
		err = startWatchApplicationComponentKinds(resController, obj, applications)
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("    process application error %s\n", err))
			}
			return err
		}
		findAllApplicationsForResource(resController, eventData.obj, applications)
	}
	resourceToBatch := batchResources{
		applications:    applications,
		nonApplications: nonApplications,
	}
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("    Sending %d applications and %d resources on channel\n", len(resourceToBatch.applications), len(resourceToBatch.nonApplications)))
	}
	resController.resourceChannel.send(&resourceToBatch)

	return nil
}
