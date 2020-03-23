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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// Check whether a resource exists
func resourceExists(resController *ClusterWatcher, resInfo resourceID) (bool, error) {
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
		_, err := intf.Get(resInfo.name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return true, nil
	}
	return false, fmt.Errorf("resourceExists Unable to find GVR for kind %s", resInfo.kind)
}

func getResource(resController *ClusterWatcher, resInfo resourceID) (*unstructured.Unstructured, error) {
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
		unstructuredObj, err := intf.Get(resInfo.name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return unstructuredObj, err
	}
	return nil, fmt.Errorf("getResource Unable to find GVR for kind %s", resInfo.kind)
}

// get kappnav status of resource
func resourcekAppNavStatus(resController *ClusterWatcher, resInfo resourceID) (string, error) {
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
		unstructuredObj, err := intf.Get(resInfo.name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		var resInfo = &resourceInfo{}
		resController.parseResource(unstructuredObj, resInfo)
		return resInfo.kappnavStatVal, nil

	}
	return "", fmt.Errorf("resourcekAppNavStatus Unable to find GVR for kind %s", resInfo.kind)
}

/*
  wait for a resource status to change until context is cancelled.
  Return true if the resource status did change before context is cancelled
  Return false otherwise
*/
func waitForkAppNavStatusChange(ctx context.Context, cancel context.CancelFunc, testName string, iteration int, resController *ClusterWatcher, resInfo resourceID, expectedStatus string) error {
	defer cancel()
	for {
		currentStat, err := resourcekAppNavStatus(resController, resInfo)
		if err != nil {
			return err
		}
		//fmt.Printf("waitForkAppNavStatusChagne %s %s %s expected: %s, current: %s\n", resInfo.kind, resInfo.namespace, resInfo.name, expectedStatus, currentStat)
		if strings.Compare(currentStat, expectedStatus) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			// cancelled
			return fmt.Errorf("timed out waiting for status: test %s, iteration %d,  %s %s %s, expected: %s current: %s", testName, iteration, resInfo.kind, resInfo.namespace, resInfo.name, expectedStatus, currentStat)
		case <-time.After(time.Millisecond * 500):
			// sleep 500 milliseconds and retry from the top
		}
	}
}

func waitForkAppNavAutoDelete(ctx context.Context, cancel context.CancelFunc, testName string, iteration int, resController *ClusterWatcher, resInfo resourceID) error {
	defer cancel()
	for {
		ok, _ := resourceExists(resController, resInfo)
		if !ok {
			return nil
		}

		select {
		case <-ctx.Done():
			// cancelled
			return fmt.Errorf("timed out waiting for resource to be auto-deleted: test %s, iteration %d,  %s %s %s", testName, iteration, resInfo.kind, resInfo.namespace, resInfo.name)
		case <-time.After(time.Millisecond * 500):
			// sleep 500 milliseconds and retry from the top
		}
	}
}

func sameAutoCreatedApplication(actual *appResourceInfo, expected *appResourceInfo) bool {
	ret := true
	if !sameLabels(actual.labels, expected.labels) {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, "sameAutoCreatedApplication labels different")
		}
		ret = false
	}

	if actual.annotations[AppAutoCreatedFromName] != expected.annotations[AppAutoCreatedFromName] {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, "sameAutoCreatedApplication kappnav.app.auto-created.from.name different")
		}
		ret = false
	}

	if !sameComponentKinds(actual.componentKinds, expected.componentKinds) {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, "sameAutoCreatedApplication componentKinds different")
		}
		ret = false
	}

	if !sameLabels(actual.matchLabels, expected.matchLabels) {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, "sameAutoCreatedApplication matchLabels different")
		}
		ret = false
	}
	if !sameMatchExpressions(actual.matchExpressions, expected.matchExpressions) {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, "sameAutoCreatedApplication matchExpressions different")
		}
		ret = false
	}
	return ret
}

func waitForkAppNavAutoCreateModify(ctx context.Context, cancel context.CancelFunc, testName string, iteration int, resController *ClusterWatcher, resInfo resourceID) error {
	defer cancel()
	for {
		createdObj, err := getResource(resController, resInfo)
		if err == nil {
			/* compare  what's created with what's expected */
			actualAutoCreated := &appResourceInfo{}
			err = resController.parseAppResource(createdObj, actualAutoCreated)
			if err != nil {
				return fmt.Errorf("auto-created resource not a valid application: test %s, iteration %d,  %s %s %s. Object: %s", testName, iteration, resInfo.kind, resInfo.namespace, resInfo.name, createdObj)
			}

			expectedAutoCreated := &appResourceInfo{}
			err = resController.parseAppResource(resInfo.resInfo.unstructuredObj, expectedAutoCreated)
			if err != nil {
				return fmt.Errorf("auto-created expected resource not a valid application: test %s, iteration %d,  %s %s %s. Object: %s", testName, iteration, resInfo.kind, resInfo.namespace, resInfo.name, resInfo.resInfo.unstructuredObj)
			}

			ok := sameAutoCreatedApplication(actualAutoCreated, expectedAutoCreated)
			if ok {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("auto-created resource has expected content: test %s, iteration %d,  %s %s %s. expected Object: %s, actual object: %s", testName, iteration, resInfo.kind, resInfo.namespace, resInfo.name, resInfo.resInfo.unstructuredObj, createdObj))
				}
				return nil
			}
			return fmt.Errorf("auto-created resource has unexpected content: test %s, iteration %d,  %s %s %s. expected Object: %s, actual object: %s", testName, iteration, resInfo.kind, resInfo.namespace, resInfo.name, resInfo.resInfo.unstructuredObj, createdObj)

		}
		select {
		case <-ctx.Done():
			// cancelled
			return fmt.Errorf("timed out waiting for resource to be auto-created: test %s, iteration %d,  %s %s %s", testName, iteration, resInfo.kind, resInfo.namespace, resInfo.name)
		case <-time.After(time.Millisecond * 500):
			// sleep 500 milliseconds and retry from the top
		}
	}
}

/* modify resource to trigger Kube into detecting a change
   this is done by addig an unit test anotation to the resource
*/
func modifyResource(resController *ClusterWatcher, resInfo resourceID) error {
	if resInfo.kind == APPLICATION && !resInfo.fileChanged {
		// don't modify application unless its backing file has changed
		return nil
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

		var unstructuredObj *unstructured.Unstructured
		var err error
		if resInfo.fileChanged {
			// fetch from file
			unstructuredObj, err = readJSON(resInfo.fileName)
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("reading modified JSON from file %s, error: %s", resInfo.fileName, err))
			}
			if err != nil {
				return err
			}
		} else {
			// fetch the current resource
			unstructuredObj, err = intf.Get(resInfo.name, metav1.GetOptions{})
			if err != nil {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("reading resource %s from Kube, error: %s", resInfo.name, err))
				}
				return err
			}
		}

		// add/update unit test timestamp annotation to trigger
		setUnitTestCounter(unstructuredObj)

		// var resInfo *resourceInfo = &resourceInfo{}
		// parseResource(unstructuredObj, resInfo);
		_, err = intf.Update(unstructuredObj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("modifyResource Unable to find GVR for kind %s", resInfo.kind)
	}

	return nil
}

// Modify the resource by setting a unit test counter
func setUnitTestCounter(unstructuredObj *unstructured.Unstructured) {
	var objMap = unstructuredObj.Object
	var metadata = objMap["metadata"].(map[string]interface{})

	annotationsInterf, ok := metadata["annotations"]
	var annotations map[string]interface{}
	if !ok {
		// annotations does not exist
		annotations = make(map[string]interface{})
		metadata["annotations"] = annotations
	} else {
		annotations = annotationsInterf.(map[string]interface{})
	}
	currentVal, ok := annotations["kappnav.unittest.counter"]
	if ok {
		// already exists. increment
		i, err := strconv.Atoi(currentVal.(string))
		if err != nil {
			annotations["kappnav.unittest.timestamp"] = "0"
		}
		annotations["kappnav.unittest.timestamp"] = strconv.Itoa(i + 1)

	} else {
		annotations["kappnav.unittest.timestamp"] = "0"
	}
}

func isAutoCreated(labels map[string]string) bool {
	return labels[AppAutoCreated] == "true"
}

/* Find what has changed between previous and current iteration. Return:
   toAdd: new resource to be created
   toDelete: old resource to be deleted
   toModify: non-application resource whose status needs to be modified
*/
func findResourceDelta(previous map[string]resourceID, current map[string]resourceID) (toAdd []resourceID, toDelete []resourceID, toModify []resourceID) {
	toAdd = make([]resourceID, 0)
	toDelete = make([]resourceID, 0)
	toModify = make([]resourceID, 0)
	for oldKey, oldResource := range previous {
		newResource, exists := current[oldKey]
		if exists {
			newResource.fileChanged = oldResource.fileName != newResource.fileName
			if (oldResource.expectedStatus != newResource.expectedStatus) || newResource.fileChanged {
				// either fileName changed, or status  changed
				toModify = append(toModify, newResource)
			}
		} else {
			// old does not exist in new. Delete
			toDelete = append(toDelete, oldResource)
		}
	}
	for newKey, newResource := range current {
		_, exists := previous[newKey]
		if !exists {
			// new resource does not exist in old. Create
			toAdd = append(toAdd, newResource)
		}
	}
	return
}

/* Wait for status to change. Return true if expected status is reached
   or false if not reached within timeout */
func waitForStatusChange(testName string, iteration int, resController *ClusterWatcher, resource resourceID, expectedStatus string) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("waitForStatusChange test: %s, iteration %d, resource: %s %s %s, expecting: %s\n", testName, iteration, resource.kind, resource.namespace, resource.name, expectedStatus))
	}
	timeout := BatchDuration * 5
	if timeout < time.Second {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	err := waitForkAppNavStatusChange(ctx, cancel, testName, iteration, resController, resource, expectedStatus)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			str := fmt.Sprintf("%s", err)
			logger.Log(CallerName(), LogTypeError, str)
		}
	}
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, "waitForStatusChange done\n")
	}
	return err
}

func waitForAutoDelete(testName string, iteration int, resController *ClusterWatcher, resource resourceID) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("waitForAutoDelete test: %s, iteration %d, resource: %s %s %s\n", testName, iteration, resource.kind, resource.namespace, resource.name))
	}
	timeout := BatchDuration * 5
	if timeout < time.Second {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	err := waitForkAppNavAutoDelete(ctx, cancel, testName, iteration, resController, resource)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			str := fmt.Sprintf("%s", err)
			logger.Log(CallerName(), LogTypeError, str)
		}
	}
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, "waitForAutoDelete done")
	}
	return err
}

func waitForAutoCreateModify(testName string, iteration int, resController *ClusterWatcher, resource resourceID) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("waitForAutoCreateModify test: %s, iteration %d, resource: %s %s %s\n", testName, iteration, resource.kind, resource.namespace, resource.name))
	}
	timeout := BatchDuration * 5
	if timeout < time.Second {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	err := waitForkAppNavAutoCreateModify(ctx, cancel, testName, iteration, resController, resource)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			str := fmt.Sprintf("%s", err)
			logger.Log(CallerName(), LogTypeError, str)
		}
	}
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, "waitForAutoCreatedModify done")
	}
	return err
}

// identity of a test reource
type resourceID struct {
	resInfo   *resourceInfo
	namespace string
	kind      string
	gvr       schema.GroupVersionResource
	name      string
	fileName  string
	// expected status
	expectedStatus string
	flyover        string
	flyoverNLS     string

	// for use by testAction only
	fileChanged bool
}

/* data structure to track data driven test iterations */
type testActions struct {
	testName string // name of test

	clusterWatcher *ClusterWatcher

	// resources for each iteration and their expected status
	resources []map[string]resourceID

	// system created resources for each iteration
	autoCreatedResources []map[string]resourceID

	kindsToCheckStatus map[string]bool

	totalIterations  int
	currentIteration int
}

func newTestActions(testName string, kindsToCheck map[string]bool) *testActions {
	var testActions = &testActions{}
	testActions.resources = make([]map[string]resourceID, 0)
	testActions.currentIteration = 0
	testActions.kindsToCheckStatus = kindsToCheck
	testActions.totalIterations = 0
	testActions.testName = testName
	return testActions
}

func (ta *testActions) setClusterWatcher(cw *ClusterWatcher) {
	ta.clusterWatcher = cw
}

func componentKey(kind, namespace, name string) string {
	return kind + "/" + namespace + "/" + name
}

/* Add data for one iteration
Note: 0th iteration is used to check initial kappnav status update for pre-populated data
*/
func (ta *testActions) addIteration(resources []resourceID, autoCreatedResources []resourceID) {
	ta.totalIterations++
	/* add test case resources */
	iterationMap := make(map[string]resourceID)
	ta.resources = append(ta.resources, iterationMap)
	for _, resource := range resources {
		iterationMap[componentKey(resource.kind, resource.namespace, resource.name)] = resource
	}

	// add controller created resources
	iterationSystemMap := make(map[string]resourceID)
	ta.autoCreatedResources = append(ta.autoCreatedResources, iterationSystemMap)
	for _, resource := range autoCreatedResources {
		iterationSystemMap[componentKey(resource.kind, resource.namespace, resource.name)] = resource
	}
}

/* perform next iteration. Return true if end has reached
 */
func (ta *testActions) transition() (bool, error) {
	if ta.currentIteration >= ta.totalIterations {
		// done
		return true, nil
	}
	err := transitionHelper(ta)
	ta.currentIteration++
	return false, err
}

func transitionHelper(ta *testActions) error {
	var previousResources map[string]resourceID
	if ta.currentIteration == 0 {
		// zeroth itertation,  just check resource status
		previousResources = ta.resources[0]
	} else {
		previousResources = ta.resources[ta.currentIteration-1]
	}

	// Calculate list of resource to add, delete, or modify
	toAdd, toDelete, toModify := findResourceDelta(previousResources, ta.resources[ta.currentIteration])

	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("test %s iteration [%d] toAdd: %d, toDelete: %d, toModify: %d\n", ta.testName, ta.currentIteration, len(toAdd), len(toDelete), len(toModify)))
	}

	// create new resources
	if ta.clusterWatcher == nil {
		return fmt.Errorf("testAction cluster watcher is nil")
	}

	fake := ta.clusterWatcher.plugin.discoveryClient
	if fake == nil {
		return fmt.Errorf("fake discovery is nil")
	}
	fakeDiscovery := fake.(*fakeDiscovery)
	err := populateResources(toAdd, ta.clusterWatcher.plugin.dynamicClient, fakeDiscovery)
	if err != nil {
		return err
	}

	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("test %s  populated resources\n", ta.testName))
	}

	// delete resources
	for _, resToDelete := range toDelete {
		var resInfo = &resourceInfo{}
		resInfo.kind = resToDelete.kind
		resInfo.gvr = resToDelete.gvr
		resInfo.namespace = resToDelete.namespace
		resInfo.name = resToDelete.name

		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("test %s  deleting %s\n", ta.testName, resInfo.name))
		}

		err := deleteResource(ta.clusterWatcher, resInfo)
		if err != nil {
			return err
		}
	}

	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("test %s  completed deletion\n", ta.testName))
	}

	// modify resources
	for _, resToModify := range toModify {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("test %s  modifying %s\n", ta.testName, resToModify.name))
		}

		err := modifyResource(ta.clusterWatcher, resToModify)
		if err != nil {
			return err
		}
	}

	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("test %s  completed modification\n", ta.testName))
	}

	// check resource status
	for _, res := range ta.resources[ta.currentIteration] {
		if ta.kindsToCheckStatus[res.kind] {
			if res.expectedStatus == NoStatus {

				if logger.IsEnabled(LogTypeInfo) {
					logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("    checking NoStatus for resource %s %s %s", res.kind, res.namespace, res.name))
				}
				/* It is difficult to check that status is not being set. The best we can do is to wait a while */
				time.Sleep(BatchDuration * 2)
				stat, err := resourcekAppNavStatus(ta.clusterWatcher, res)
				if err != nil {
					return err
				}
				/* when NoStatus is expected, the getStatus() method should not even be called to get status
				   . If it gets called, it will return "NoStatus", and the check for no status (with value "") fails */
				if stat != "" {
					err = fmt.Errorf("    resource %s %s %s has unexpected status. Expecting no status, but has %s", res.kind, res.namespace, res.name, stat)
					return err
				}

				if logger.IsEnabled(LogTypeInfo) {
					logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("    resource %s %s %s has expected no status\n", res.kind, res.namespace, res.name))
				}
			} else {
				// check status for this resource
				err = waitForStatusChange(ta.testName, ta.currentIteration, ta.clusterWatcher, res, res.expectedStatus)
				if err != nil {
					return err
				}

				if logger.IsEnabled(LogTypeInfo) {
					logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("    resource %s %s %s has expected status %s\n", res.kind, res.namespace, res.name, res.expectedStatus))
				}
			}
		}
	}

	// Calculate list of resource the controller should auto add, modify, or delte
	var previousSystemResources map[string]resourceID
	if ta.currentIteration == 0 {
		// zeroth itertation
		previousSystemResources = make(map[string]resourceID)
	} else {
		previousSystemResources = ta.autoCreatedResources[ta.currentIteration-1]
	}
	toAutoCreate, toAutoDelete, toAutoModify := findResourceDelta(previousSystemResources, ta.autoCreatedResources[ta.currentIteration])

	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("test %s iteration system resources: [%d] toAdd: %d, toDelete: %d, toModify: %d\n", ta.testName, ta.currentIteration, len(toAdd), len(toDelete), len(toModify)))
	}

	// wait for auto-deleted resources to be gone
	for _, resToAutoDelete := range toAutoDelete {
		err := waitForAutoDelete(ta.testName, ta.currentIteration, ta.clusterWatcher, resToAutoDelete)
		if err != nil {
			return err
		}
	}

	// wait for new resource to be auto-created
	for _, resToAutoCreate := range toAutoCreate {
		err := waitForAutoCreateModify(ta.testName, ta.currentIteration, ta.clusterWatcher, resToAutoCreate)
		if err != nil {
			return err
		}
	}

	// wait for new resource to be modified
	for _, resToAutoModify := range toAutoModify {
		err := waitForAutoCreateModify(ta.testName, ta.currentIteration, ta.clusterWatcher, resToAutoModify)
		if err != nil {
			return err
		}
	}

	// check status of remaining auto-created resoruces
	for _, res := range ta.autoCreatedResources[ta.currentIteration] {
		if ta.kindsToCheckStatus[res.kind] {
			// check status for this resource
			err = waitForStatusChange(ta.testName, ta.currentIteration, ta.clusterWatcher, res, res.expectedStatus)
			if err != nil {
				return err
			}

			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("    resource %s %s %s has expected status %s\n", res.kind, res.namespace, res.name, res.expectedStatus))
			}
		}
	}

	return nil
}

// perform all transitions
func (ta *testActions) transitionAll() error {
	var done bool
	var err error
	for err == nil && !done {
		done, err = ta.transition()
	}
	return err
}

/* Get current status of a resource. This is the part of the scaffolding
   that replaces calculateComponentStatusFunc so for the unit test
   to return the exepcted status of a resource
*/
func (ta *testActions) getStatus(kind string, namespace string, name string) (status string, flyover string, flyoverNLS string, err error) {
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%s %s %s %s %d %d\n", ta.testName, kind, namespace, name, ta.currentIteration, ta.totalIterations))
	}
	if ta.totalIterations == 0 {
		// for tests that don't have iteration data
		return "", "", "", fmt.Errorf("total iterations == 0")
	}

	iteration := ta.currentIteration
	if iteration >= ta.totalIterations {
		// At the end. Background thread may still be processing last iteration  data
		iteration = ta.totalIterations - 1
	}
	key := componentKey(kind, namespace, name)
	if res, ok := ta.resources[iteration][key]; ok {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%d %s, %s, %s, %s\n", ta.currentIteration, kind, namespace, name, res.expectedStatus))
		}
		return res.expectedStatus, res.flyover, res.flyoverNLS, nil
	}

	return "", "", "", fmt.Errorf("Can't get status for unknown resource %s", key)
}
