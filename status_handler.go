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
	"k8s.io/klog"
)

// Send resource status change back to Kubernetes server
func sendResourceStatus(resController *ClusterWatcher, resInfo *resourceInfo, status string, flyoverText string, flyOverNLS string) error {
	if klog.V(4) {
		klog.Infof("sendResourceStatus %s set to %s\n", resInfo.name, status)
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
		var unstructuredObj *unstructured.Unstructured
		var err error
		unstructuredObj, err = intf.Get(resInfo.name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		var resInfo = &resourceInfo{}
		parseResource(unstructuredObj, resInfo)
		if strings.Compare(resInfo.kappnavStatVal, status) != 0 {
			// change status
			if klog.V(2) {
				klog.Infof("Setting kappnav status on Kubernetes server: resource: %s %s %s,  status: %s, flyover: %s\n", resInfo.kind, resInfo.namespace, resInfo.name, status, flyoverText)
			}
			setkAppNavStatus(unstructuredObj, status, flyoverText, flyOverNLS)
			_, err = intf.Update(unstructuredObj, metav1.UpdateOptions{})
			if err != nil {
				if klog.V(2) {
					klog.Errorf("    error setting kappnav status %s\n", err)
				}
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("Unable to find GVR for kind %s", resInfo.kind)
}

type statusChecker struct {
	count         map[string]int // counter number of each different status
	precedence    []string       // precedence
	unknownStatus string         // value of unknown status
}

// Return a new Status checker
func newStatusChecker(precedence []string, unkownStatus string) *statusChecker {
	var checker = statusChecker{}

	checker.unknownStatus = unkownStatus
	checker.precedence = precedence
	checker.count = make(map[string]int)
	for _, value := range precedence {
		checker.count[value] = 0
	}
	return &checker
}

// Add another status to be checked
// Return :
//    valid: true if the status value is valid
//    alreadyHighest: indication whether current calculated status is already
//       highest precedence status.  Caller need not continue checking in
//       that case
func (checker *statusChecker) addStatus(status string) (valid bool, alreadyHighest bool) {
	// status is unknown
	if status == "" {
		status = checker.unknownStatus
	}
	value, ok := checker.count[status]
	if !ok {
		return false, false
	}
	checker.count[status] = value + 1
	if status == checker.precedence[0] {
		return true, true
	}
	return true, false
}

// Return the final status
func (checker *statusChecker) finalStatus() string {
	var statusPrecedence = checker.precedence
	for _, value := range statusPrecedence {
		if checker.count[value] > 0 {
			return value
		}
	}

	/* no status on anything. Return unkown status
	 */
	return checker.unknownStatus
}

// Process one batch of changes
func processBatchOfApplicationsAndResources(ts *batchStore, resources *batchResources) error {

	apps := make([]string, 0, len(resources.applications))
	for appName := range resources.applications {
		apps = append(apps, appName)
	}
	if klog.V(4) {
		klog.Infof("    processBatchOfApplicationAndResources applications: total: %d, application names: %s\n", len(resources.applications), apps)
	}

	hasStatus := make(map[string]*resourceInfo)
	toChange := make(map[string]*resourceInfo)

	// calculate application status for all affected applications
	for _, res := range resources.applications {
		visited := make(map[string]*resourceInfo)
		_, stat, err := processOneApplication(ts.resController, res, visited, hasStatus, resources.nonApplications, toChange)
		if err != nil {
			return err
		}
		key := res.key()
		if res.kappnavStatVal != stat {
			// status changed
			newRes := &resourceInfo{}
			*newRes = *res
			newRes.kappnavStatVal = stat
			toChange[key] = newRes
			hasStatus[key] = newRes
		} else {
			hasStatus[key] = res
		}
	}

	// calculate resource status for non-application resources not yet processed
	for _, resInfo := range resources.nonApplications {
		// calculate resource status
		_, err := processOneResource(ts.resController, resInfo, hasStatus, resources.nonApplications, toChange)
		if err != nil {
			return err
		}
	}

	// update kappnav status for all resources whose status have changed
	for _, res := range toChange {
		err := sendResourceStatus(ts.resController, res, res.kappnavStatVal, res.flyOver, res.flyOverNLS)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
Process status for one application
 res: the application
 visited: application already visited when computing status for one top level application
 hasStatus: accumulated resources with known status
 toFetch: call API server to fetch current status. If not in this set, fetch status from cache.
 toChange: accumulated resources with status change. call API server to update status

 Return:
   statusOK: true if OK, false to skip this application to avoid infinite recursion
   status: the status of the application
   processErr : any error captured
*/
func processOneApplication(resController *ClusterWatcher, res *resourceInfo, visited map[string]*resourceInfo, hasStatus map[string]*resourceInfo, toFetch map[string]*resourceInfo, toChange map[string]*resourceInfo) (statusOK bool, status string, processErr error) {
	if klog.V(4) {
		klog.Infof("processOneApplication for %s\n", res.name)
	}

	key := res.key()
	_, ok := visited[key]
	if ok {
		if klog.V(4) {
			klog.Infof("    application %s already visited\n", res.name)
		}
		// already visited
		return false, "", nil
	}
	visited[key] = res

	computed, exists := hasStatus[key]
	if exists {
		// return the already computed status
		if klog.V(4) {
			klog.Infof("    application %s already has status %s\n", computed.name, computed.kappnavStatVal)
		}
		return true, computed.kappnavStatVal, nil
	}

	obj := res.unstructuredObj
	appInfo := &appResourceInfo{}
	resController.parseAppResource(obj, appInfo)

	checker := newStatusChecker(resController.getStatusPrecedence(), resController.unknownStatus)
	var componentKinds = appInfo.componentKinds
	// loop over all components kinds
	for _, component := range componentKinds {
		// loop over all resources of each component kind
		gvr, ok := resController.getGVRForGroupKind(component.group, component.kind)
		var resources = resController.listResources(gvr)
		for _, res := range resources {

			var unstructuredObj = res.(*unstructured.Unstructured)

			var resInfo = &resourceInfo{}
			parseResource(unstructuredObj, resInfo)
			if resourceComponentOfApplication(resController, appInfo, resInfo) {
				// not self and labels match selector
				if klog.V(4) {
					klog.Infof("    found component: %s\n", resInfo.name)
				}

				var stat string
				var err error
				if resInfo.kind == APPLICATION {
					// recursively calculate application status
					var tmpAppInfo = &appResourceInfo{}
					err = resController.parseAppResource(unstructuredObj, tmpAppInfo)
					if err != nil {
						return false, "", err
					}
					ok, stat, err = processOneApplication(resController, &tmpAppInfo.resourceInfo, visited, hasStatus, toFetch, toChange)
					if err != nil {
						return false, "", err
					}
					if !ok {
						// skip this one to avoid infinite recursion
						if klog.V(4) {
							klog.Infof("    skipping application: %s\n", resInfo.name)
						}
						continue
					}
				} else {
					// calculate resource status
					stat, err = processOneResource(resController, resInfo, hasStatus, toFetch, toChange)
					if err != nil {
						return false, stat, err
					}

				}
				checker.addStatus(stat)
			}
		}
	}
	status = checker.finalStatus()

	if klog.V(4) {
		klog.Infof("    processOneApplication final status for application %s %s %s is %s\n", appInfo.kind, appInfo.namespace, appInfo.name, status)
	}
	return true, status, nil
}

/* Process status update for one non-application resource
   resInfo: resource for which to compute status
   hasStatus:  resources that already has computed status
   toFetch: resources whose status need to be computed from API server
   toChange: resources whose status have changed, need to update API server
*/
func processOneResource(resController *ClusterWatcher, resInfo *resourceInfo, hasStatus map[string]*resourceInfo, toFetch map[string]*resourceInfo, toChange map[string]*resourceInfo) (string, error) {
	key := resInfo.key()
	if res, ok := hasStatus[key]; ok {
		// status already computed
		if klog.V(4) {
			klog.Infof("processOneResource status already computed  %s %s %s\n", resInfo.kind, resInfo.namespace, resInfo.name)
		}
		return res.kappnavStatVal, nil
	}
	_, ok := toFetch[key]
	if ok {
		// Resource has changed. Compute status from api Server
		if klog.V(4) {
			klog.Infof("processOneResource fetching status for %s %s %s\n", resInfo.kind, resInfo.namespace, resInfo.name)
		}
		stat, flyover, flyoverNLS, err := resController.plugin.statusFunc(apiURL, resInfo.kind, resInfo.namespace, resInfo.name)
		if err != nil {
			if klog.V(4) {
				klog.Infof("processOneResource error fetching status for %s %s %s\n", resInfo.kind, resInfo.namespace, resInfo.name)
				klog.Infof("%v\n", err)
			}
			return stat, err
		}
		if stat != resInfo.kappnavStatVal || flyover != resInfo.flyOver || flyoverNLS != resInfo.flyOverNLS {
			newRes := &resourceInfo{}
			*newRes = *resInfo
			newRes.kappnavStatVal = stat
			newRes.flyOver = flyover
			newRes.flyOverNLS = flyoverNLS
			toChange[key] = newRes
			hasStatus[key] = newRes
		} else {
			hasStatus[key] = resInfo
		}
		return stat, nil
	}
	// Resource has not changed. Use pre-existig status
	hasStatus[key] = resInfo
	return resInfo.kappnavStatVal, nil
}
