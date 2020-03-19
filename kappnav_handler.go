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
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//KappnavResourceInfo contains info from a KAppNav resource
type KappnavResourceInfo struct {
	okdFeaturedApp string
	okdAppLauncher string
}

// KAppNavHandler processes changes to the KAppNav custom resource
var KAppNavHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, "Called for kind KAppNav")
	}
	key := eventData.key
	_, exists, err := rw.store.GetByKey(key)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("fetching key %s failed: %v", key, err))
		}
		return err
	}
	if !exists {
		// KAppNav kind has been deleted
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, "KAppNav kind has been deleted")
		}
		resController.deleteGVR(eventData.obj)
	} else {
		// KAppNav resource added, modified, deleted
		if eventData.funcType == AddFunc {
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, "Event type: add")
			}
			addKappnav(eventData)
		} else if eventData.funcType == UpdateFunc {
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, "Event type: update")
			}
			updateKappnav(eventData)
		} else if eventData.funcType == DeleteFunc {
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, "Event type: delete")
			}
			deleteKappnav(eventData)
		}
	}
	return nil
}

// addKappnav is called when the KAppNav resource is created
func addKappnav(eventData *eventHandlerData) error {
	return updateKappnav(eventData)
}

// addKappnav is called when the KAppNav resource is modified
func updateKappnav(eventData *eventHandlerData) error {
	kubeEnv := os.Getenv("KUBE_ENV")
	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, "KUBE_ENV = "+kubeEnv)
	}
	if kubeEnv == "okd" || kubeEnv == "ocp" || kubeEnv == "minishift" {
		if isLatestOKD {
			// TODO: Handle Latest/OpenShiftWebConsoleConfig case
			//cl, err := client.New(cfg, client.Options{})
		} else {
			var kappnavInfo = &KappnavResourceInfo{}
			parseKAppNavResource(eventData.obj, kappnavInfo)
		}
	}
	return nil
}

// deleteKappnav is called when the KAppNav resource is deleted
func deleteKappnav(eventData *eventHandlerData) error {
	kubeEnv := os.Getenv("KUBE_ENV")
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, "KUBE_ENV = "+kubeEnv)
	}
	if kubeEnv == "okd" || kubeEnv == "ocp" {
		if isLatestOKD {
			// TODO: Handle Latest/OpenShiftWebConsoleConfig case
			//cl, err := client.New(cfg, client.Options{})
		} else {
			var kappnavInfo = &KappnavResourceInfo{}
			parseKAppNavResource(eventData.obj, kappnavInfo)
		}
	}
	return nil
}

// parseKAppNavResource extracts selected KAppNav resource fields to a structure
func parseKAppNavResource(resource interface{}, kappnavResource *KappnavResourceInfo) error {
	unstructuredObj := resource.(*unstructured.Unstructured)
	var objMap = unstructuredObj.Object
	var spec map[string]interface{}
	tmp, ok := objMap[SPEC]
	if !ok {
		return fmt.Errorf("object has no spec %s", unstructuredObj)
	}
	spec = tmp.(map[string]interface{})

	// get the logging map from kappnav resource "spec" field
	loggingMap, _ := spec["logging"]
	// retrieve logging value of "controller" from logging map
	if loggingMap != nil {
		loggingValue := loggingMap.(map[string]interface{})["controller"].(string)
		if len(loggingValue) > 0 {
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, "Set the log level to "+loggingValue)
			}
			//invoke setLoggingLevel to reset log level
			SetLoggingLevel(loggingValue)
		}
	}

	tmp1, _ := spec["console"]
	if tmp1 != nil {
		console := tmp1.(map[string]interface{})
		kappnavResource.okdAppLauncher = console["okdAppLauncher"].(string)
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("KAppNav resource spec.console.okdAppLauncher: %v", kappnavResource.okdAppLauncher))
		}
		kappnavResource.okdFeaturedApp = console["okdFeaturedApp"].(string)
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("KAppNav resource spec.console.okdFeaturedApp: %v", kappnavResource.okdFeaturedApp))
		}
	}

	return nil
}

// addString adds a string to an array if the array
// does not already contain the string
func addString(input []interface{}, str string) []interface{} {
	if containsString(input, str) {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Output (unchanged) = %v", input))
		}
		return input
	}
	output := append(input, str)

	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Output = %v", output))
	}

	return output
}

// removeString removes all instances of a string from an array
func removeString(input []interface{}, str string) []interface{} {
	i := 0
	for _, value := range input {
		if value == str {
			i++
		}
	}
	if i == 0 {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Output (unchanged) = %v", input))
		}
		return input
	}
	output := make([]interface{}, len(input)-i)
	i = 0
	for _, value := range input {
		if value != str {
			output[i] = value
			i++
		}
	}

	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Output = %v", output))
	}

	return output
}

func containsString(input []interface{}, str string) bool {
	for _, value := range input {
		if value == str {
			return true
		}
	}
	return false
}
