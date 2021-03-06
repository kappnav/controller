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
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (

	// AppAutoCreate ...
	AppAutoCreate = "kappnav.app.auto-create"
	// AppAutoCreateName ...
	AppAutoCreateName = "kappnav.app.auto-create.name"
	// AppAutoCreateKinds ...
	AppAutoCreateKinds = "kappnav.app.auto-create.kinds"
	// AppAutoCreateVersion ...
	AppAutoCreateVersion = "kappnav.app.auto-create.version"
	// AppAutoCreateLabel ...
	AppAutoCreateLabel = "kappnav.app.auto-create.label"
	// AppAutoCreateLabelValues ...
	AppAutoCreateLabelValues = "kappnav.app.auto-create.labels-values"
	// AppAutoCreated ...
	AppAutoCreated = "kappnav.app.auto-created"

	/* labels for auto-created app */
	labelName       = "app.kubernetes.io/name"
	labelVersion    = "app.kubernetes.io/version"
	labelAutoCreate = "kappnav.app.auto-created"

	defaultAutoCreateAppVersion = "1.0.0"
	defaultAutoCreateAppLabel   = "app"

	// AppPartOf ...
	AppPartOf = "app.kubernetes.io/part-of"

	namespaceRegex = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"

)

/* default kinds for auto-created app*/
var defaultKinds = []groupKind{
	groupKind{group: "apps", kind: "Deployment"},
	groupKind{group: "apps", kind: "StatefulSet"},
	groupKind{group: "", kind: "Service"},
	groupKind{group: "extensions", kind: "Ingress"},
	groupKind{group: "", kind: "ConfigMap"}}

/* information about resource from which to auto-create applications */
type autoCreateResourceInfo struct {
	resourceInfo
	autoCreate            bool
	autoCreateName        string
	autoCreateKinds       []groupKind
	autoCreateVersion     string
	autoCreateLabel       string
	autoCreateLabelValues []string
	partOfLabel           string
	orgPartOfLabel        string
}

/* map to track app name being added or deleted for an auto creation name */
var autoCreateAppMap = make(map[string]map[string][]string)

/* Parse a comma separated string into array of strings. Leading and trailing spaces are removed
   Examples:
       instr: " A , B, C D  "
       output:  []string { "A", "B", "C D" }
*/
func stringToArrayOfString(inStr string) []string {
	tmpArray := strings.Split(inStr, ",")
	var finalArray = make([]string, 0)
	for _, str := range tmpArray {
		str = strings.Trim(str, " ")
		if len(str) != 0 {
			finalArray = append(finalArray, str)
		}
	}
	return finalArray
}

func testNonAlphaNumeric(r rune) bool {
	if (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') {
		return false
	}
	return true
}

func isStringAlphaNumeric(str string) bool {
	for _, r := range str {
		if testNonAlphaNumeric(r) {
			return false
		}
	}
	return true
}

/* Trim leading and trailing non-alphanumeric from string.
   Return new string if the remainder is alphnumeric. and not empty
    ok == true if a valid string can be returned.
*/
func trimNonAlphaNumeric(str string) (string, bool) {
	newStr := strings.TrimFunc(str, testNonAlphaNumeric)
	if newStr == "" {
		return "", false
	}
	if isStringAlphaNumeric(newStr) {
		return newStr, true
	}
	return "", false
}

/* Convert a comma separate string into array of alphanumeric
First, the string is split by ","
Then all trailing and ending non-alpha numeric are removed
Any string that is not all alpha-numeric is discarded.
An array is constructed of remaining strings.
*/
func stringToArrayOfAlphaNumeric(inStr string) []string {
	tmpArray := stringToArrayOfString(inStr)
	var ret = make([]string, 0)
	for _, val := range tmpArray {
		if newval, ok := trimNonAlphaNumeric(val); ok {
			ret = append(ret, newval)
		}
	}
	return ret
}

// stringToNamespaceSet converts a comma separated string into a
// set of strings. Only valid kube namespace names will be added
// to the set.
// (the "set" is a map of strings to themselves as go has not set datatype.)
func stringToNamespaceSet(inStr string) map[string]string {
	strings := stringToArrayOfString(inStr)
	ret := make(map[string]string)
	for _, val := range strings {
		isValidName, _ := regexp.MatchString(namespaceRegex, val)
		if isValidName {
			ret[val] = val
		} else {
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf(" %s is not a valid namespace name", val))
			}
		}
	}
	return ret
}

func stringToArrayOfLabelValues(inStr string) []string {
	tmpArray := stringToArrayOfString(inStr)
	for index, val := range tmpArray {
		tmpArray[index] = toLabelName(val)
	}
	return tmpArray
}

/* Parse a resource from which to auto create applications
   unstructuredObj: object to be parsed
   Return an autoCreateResourceInfo  to represent the resource
        Return  nil if the label kappnav.app.auto-create is not set to true.
       Note: If parsing error occurs for annotations, default values are used.
*/
func (resController *ClusterWatcher) parseAutoCreateResourceInfo(unstructuredObj *unstructured.Unstructured) *autoCreateResourceInfo {
	resourceInfo := &autoCreateResourceInfo{}
	resController.parseResource(unstructuredObj, &resourceInfo.resourceInfo)

	// check if part-of label set
	_, pok := resourceInfo.resourceInfo.labels[AppPartOf]

	// check if auto-create is enabled
	autoCreate, aok := resourceInfo.resourceInfo.labels[AppAutoCreate]

	// auto-create not enabled and part-of label not set
	if !aok && !pok {
		return nil
	}

	// no part-of and auto-create set to false
	if !pok && autoCreate != "true" {
		return nil
	}

	// retrieve part-of label
	labelsObj, ok := resourceInfo.metadata[LABELS]
	var labels map[string]interface{}
	if ok {
		labels = labelsObj.(map[string]interface{})
	} else {
		labels = make(map[string]interface{})
	}
	
	tmpP, ok := labels[AppPartOf] //ignore !ok and it shuold not happen
	if ok {
		resourceInfo.partOfLabel, ok = tmpP.(string)
		if ok {
			// original part-of label value
			resourceInfo.orgPartOfLabel = resourceInfo.partOfLabel
			// converted part-of value to lower case
			resourceInfo.partOfLabel = toDomainName(resourceInfo.partOfLabel)		
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("original partOfLabel: %s, converted value: %s", resourceInfo.orgPartOfLabel, resourceInfo.partOfLabel))
			}
		}
	}

	// retrieve auto-create annotations
	annotationsObj, ok := resourceInfo.metadata[ANNOTATIONS]
	var annotations map[string]interface{}
	if ok {
		annotations = annotationsObj.(map[string]interface{})
	} else {
		annotations = make(map[string]interface{})
	}
	tmp, ok := annotations[AppAutoCreateName]
	if !ok && autoCreate == "true" {
		// use default.
		// TODO: Transform it due to syntax restriction
		resourceInfo.autoCreateName = resourceInfo.name
	} else {
		resourceInfo.autoCreateName, ok = tmp.(string)
		if !ok && autoCreate == "true" {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Metadata %s for resource %s/%s is not a string. Using default.", AppAutoCreateName, resourceInfo.resourceInfo.namespace, resourceInfo.resourceInfo.name))
			}
			resourceInfo.autoCreateName = resourceInfo.name
		} else {		
			resourceInfo.autoCreateName = toDomainName(resourceInfo.autoCreateName)
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("autoCreateName: %s", resourceInfo.autoCreateName))
			}
		}
	}

	tmp, ok = annotations[AppAutoCreateKinds]
	// if AppAutoCreateKinds annotation not specified, check if it is configured in kappnav CR
	if !ok {
		resourceInfo.autoCreateKinds = getAutoCreateKindsFromKappnav(resController)
		if len(resourceInfo.autoCreateKinds) == 0 {
			resourceInfo.autoCreateKinds = defaultKinds
		}
	} else {
		autoCreateKindsStr, ok := tmp.(string)
		if !ok {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Metadata %s for resource %s/%s is not a string of comma separated kinds. Using default.", AppAutoCreateKinds, resourceInfo.resourceInfo.namespace, resourceInfo.resourceInfo.name))
			}
			resourceInfo.autoCreateKinds = getAutoCreateKindsFromKappnav(resController)
			if len(resourceInfo.autoCreateKinds) == 0 {
				resourceInfo.autoCreateKinds = defaultKinds
			}
		} else {
			arrayOfString := stringToArrayOfAlphaNumeric(autoCreateKindsStr)
			if len(arrayOfString) == 0 {
				if logger.IsEnabled(LogTypeError) {
					logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Metadata %s for resource %s/%s does not contain comma separated kinds. Its current value is %s. Switching to default.", AppAutoCreateKinds, resourceInfo.resourceInfo.namespace, resourceInfo.resourceInfo.name, autoCreateKindsStr))
				}
				resourceInfo.autoCreateKinds = getAutoCreateKindsFromKappnav(resController)
				if len(resourceInfo.autoCreateKinds) == 0 {
					resourceInfo.autoCreateKinds = defaultKinds
				}
			} else {
				/* Transform array of string to array of groupKind */
				resourceInfo.autoCreateKinds = make([]groupKind, 0, len(arrayOfString))
				for _, val := range arrayOfString {
					var group string
					var kind string
					var gvr schema.GroupVersionResource
					// a / delimiter indicates the kappnav.app.auto-create.kinds anno entry is group/kind
					split := strings.Split(val, "/")
					if len(split) > 1 {
						group = split[0]
						kind = split[1]
						if logger.IsEnabled(LogTypeDebug) {
							logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Using group: %s kind: %s from kappnav.app.auto-create.kinds entry: %s", group, kind, val))
						}
						gvr, ok = resController.getWatchGVRForKind(kind)
					} else {
						// no / so kappnav.app.auto-create.kinds anno entry is just a kind
						kind = val
						gvr, ok = resController.getWatchGVRForKind(kind)
						if ok {
							group = gvr.Group
							if logger.IsEnabled(LogTypeDebug) {
								logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Using group: %s from watch GVR for kind: %s", group, kind))
							}
						} else {
							gvr, ok = coreKindToGVR[val]
							if ok {
								group = gvr.Group
								if logger.IsEnabled(LogTypeDebug) {
									logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Using group: #s from default GVR for core kind: %s, using default group: App", val))
								}
							} else {
								group = "apps"
								if logger.IsEnabled(LogTypeDebug) {
									logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("No GVR found for kind: %s, using default group: apps", val))
								}
							}
						}
					}
					if (gvr != schema.GroupVersionResource{}) {
						resourceInfo.autoCreateKinds = append(resourceInfo.autoCreateKinds, groupKind{group, val, nil})
					} else {
						if logger.IsEnabled(LogTypeDebug) {
							logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("No GVR found for kappnav.app.auto-create.kinds entry: %s, skipping", val))
						}
					}
				}
			}
		}
	}
	tmp, ok = annotations[AppAutoCreateVersion]
	if !ok {
		resourceInfo.autoCreateVersion = defaultAutoCreateAppVersion
	} else {
		resourceInfo.autoCreateVersion, ok = tmp.(string)
		if !ok {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Metadata %s for resources %s/%s is not a string. Using default.", AppAutoCreateVersion, resourceInfo.resourceInfo.namespace, resourceInfo.resourceInfo.name))
			}
			resourceInfo.autoCreateVersion = defaultAutoCreateAppVersion
		}
	}
	tmp, ok = annotations[AppAutoCreateLabel]
	if !ok {
		resourceInfo.autoCreateLabel = defaultAutoCreateAppLabel
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Using default %v", resourceInfo))
		}
	} else {
		resourceInfo.autoCreateLabel, ok = tmp.(string)
		if !ok {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Metadata %s for resoruce %s/%s is not a string. Using default.", AppAutoCreateLabel, resourceInfo.resourceInfo.namespace, resourceInfo.resourceInfo.name))
			}
			resourceInfo.autoCreateLabel = defaultAutoCreateAppLabel
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Using default2 %v", resourceInfo))
			}
		} else {
			resourceInfo.autoCreateLabel = toLabel(resourceInfo.autoCreateLabel)
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Using default3 %v", resourceInfo.autoCreateLabel))
			}
		}
	}
	tmp, ok = annotations[AppAutoCreateLabelValues]
	if !ok {
		resourceInfo.autoCreateLabelValues = []string{resourceInfo.autoCreateName}
		// use part-of label if autoCreateLabelValues not set
		if len(resourceInfo.partOfLabel) > 0 {
			resourceInfo.autoCreateLabelValues = []string{resourceInfo.partOfLabel}
		}
	} else {
		autoCreateValuesStr, ok := tmp.(string)
		if !ok {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Metadata %s for resource %s/%s does not contain comma separted label values. Using default.", AppAutoCreateLabelValues, resourceInfo.resourceInfo.namespace, resourceInfo.resourceInfo.name))
			}
			resourceInfo.autoCreateLabelValues = []string{resourceInfo.autoCreateName}
		} else {
			labelValues := stringToArrayOfLabelValues(autoCreateValuesStr)
			if len(labelValues) == 0 {
				if logger.IsEnabled(LogTypeError) {
					logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Metadata %s for resource %s/%s does not contain comma separated label values. Its current value: %s.  Using default.", AppAutoCreateLabelValues, resourceInfo.resourceInfo.namespace, resourceInfo.resourceInfo.name, autoCreateValuesStr))
				}
				resourceInfo.autoCreateLabelValues = []string{resourceInfo.autoCreateName}
			} else {
				resourceInfo.autoCreateLabelValues = labelValues
				// check if partOf label is set in autoCreateLabel
				if len(resourceInfo.autoCreateLabel) > 0 && resourceInfo.autoCreateLabel == AppPartOf && len(resourceInfo.autoCreateLabelValues) == 1 {
					resourceInfo.partOfLabel = autoCreateValuesStr
				}
			}
		}
	}
	return resourceInfo
}

/*
 * Handle auto creation of application from resources
 */
var autoCreateAppHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	key := eventData.key
	_, exists, err := rw.store.GetByKey(key)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("fetching key %s from store failed: %v", key, err))
		}
		return err
	}

	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Processing %d current object: %s", eventData.funcType, eventData.obj.(*unstructured.Unstructured)))
	}

	if eventData.funcType == UpdateFunc {
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Processing %d old object : %s", eventData.funcType, eventData.oldObj.(*unstructured.Unstructured)))
		}
	}

	resourceInfo := resController.parseAutoCreateResourceInfo(eventData.obj.(*unstructured.Unstructured))

	if resourceInfo == nil {
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Not a resource to auto-create"))
		}
		// not an auto-create resource
		if eventData.funcType == UpdateFunc {
			oldResInfo := resController.parseAutoCreateResourceInfo(eventData.oldObj.(*unstructured.Unstructured))
			if oldResInfo != nil {
				/* Went from auto-created to not auto-created */
				if logger.IsEnabled(LogTypeInfo) {
					logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Applications no longer auto-created for resource %s, kind: %s. Deleting existing auto-created application", key, oldResInfo.kind))
				}
				if len(oldResInfo.autoCreateName) > 0 {
					deleteAutoCreatedApplicationsForResource(resController, rw, oldResInfo, oldResInfo.autoCreateName)
				}
				if len(oldResInfo.partOfLabel) > 0 {
					deleteAutoCreatedApplicationsForResource(resController, rw, oldResInfo, oldResInfo.partOfLabel)
				}				
			}
		}
		return nil
	}

	if !exists {
		// resource deleted
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    processing deleted resource %s", key))
		}
		// Delete auto-created applications
		if len(resourceInfo.autoCreateName) > 0 {
			deleteAutoCreatedApplicationsForResource(resController, rw, resourceInfo, resourceInfo.autoCreateName)
		}
		if len(resourceInfo.partOfLabel) > 0 {
			deleteAutoCreatedApplicationsForResource(resController, rw, resourceInfo, resourceInfo.partOfLabel)
		}
	} else if eventData.funcType == UpdateFunc || eventData.funcType == AddFunc {
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("processing add/update resource : %s, kind: %s", key, resourceInfo.kind))
		}
		if eventData.funcType == UpdateFunc {
			oldResInfo := resController.parseAutoCreateResourceInfo(eventData.oldObj.(*unstructured.Unstructured))

			if oldResInfo != nil {
				if (len(oldResInfo.autoCreateName) > 0 && len(resourceInfo.autoCreateName) > 0 && oldResInfo.autoCreateName != resourceInfo.autoCreateName) ||
					(len(oldResInfo.partOfLabel) > 0 && len(resourceInfo.partOfLabel) > 0 && oldResInfo.partOfLabel != resourceInfo.partOfLabel) {
					//name of auto created resource changed. Delete the old one
					if len(resourceInfo.autoCreateName) > 0 {
						if logger.IsEnabled(LogTypeInfo) {
							logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Auto-created application for resource kind %s, name %s changed from %s to %s. Deleting existing auto-created application", resourceInfo.kind, key, oldResInfo.autoCreateName, resourceInfo.autoCreateName))
						}
						deleteAutoCreatedApplicationsForResource(resController, rw, oldResInfo, oldResInfo.autoCreateName)
					}
					if len(resourceInfo.partOfLabel) > 0 {
						if logger.IsEnabled(LogTypeInfo) {
							logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Auto-created application for resource kind %s, name %s changed from %s to %s. Deleting existing auto-created application", resourceInfo.kind, key, oldResInfo.partOfLabel, resourceInfo.partOfLabel))
						}
						deleteAutoCreatedApplicationsForResource(resController, rw, oldResInfo, oldResInfo.partOfLabel)
					}			
				} else {
					// Update autoCreateName array in case resource is changed
					if len(resourceInfo.autoCreateName) > 0 {
						updateAutoCreateAppNameMap(resourceInfo, "add", resourceInfo.autoCreateName)
					}
					if len(resourceInfo.partOfLabel) > 0 {
						updateAutoCreateAppNameMap(resourceInfo, "add", resourceInfo.partOfLabel)
					}			
				}
			}
		} else { //AddFunc
			// Update autoCreateNameMap when an auto-created app is added			
			if len(resourceInfo.autoCreateName) > 0 {
				updateAutoCreateAppNameMap(resourceInfo, "add", resourceInfo.autoCreateName)
			}
			if len(resourceInfo.partOfLabel) > 0 {
				updateAutoCreateAppNameMap(resourceInfo, "add", resourceInfo.partOfLabel)
			}
		}

		//create or modify app
		if len(resourceInfo.autoCreateName) > 0 {
			err := autoCreateModifyApplication(resController, rw, resourceInfo, resourceInfo.autoCreateName)
			if err != nil {
				if logger.IsEnabled(LogTypeError) {
					logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error processing processig add/update resource %s: %s", key, err))
				}
				return err
			}
		}
		if len(resourceInfo.partOfLabel) > 0 {
			err := autoCreateModifyApplication(resController, rw, resourceInfo, resourceInfo.partOfLabel)
			if err != nil {
				if logger.IsEnabled(LogTypeError) {
					logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error processing processig add/update resource %s: %s", key, err))
				}
				return err
			}
		}

	} else {
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, "Skipping event")
		}
	}
	return nil
}

func deleteAutoCreatedApplicationsForResource(resController *ClusterWatcher, rw *ResourceWatcher, resInfo *autoCreateResourceInfo, nameKey string) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("Deleting auto-created application %s/%s", resInfo.namespace, resInfo.autoCreateName))
	}

	gvr, ok := resController.getWatchGVR(coreApplicationGVR)
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
		unstructuredObj, err = intf.Get(nameKey, metav1.GetOptions{})
		if err != nil {
			/* Most likely resource does not exist */
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error getting application to be deleted %s/%s. Error:%s", resInfo.namespace, nameKey, err))
			}
			return nil
		}

		var appResInfo = &appResourceInfo{}
		resController.parseAppResource(unstructuredObj, appResInfo)

		//delete app name from autoCreateName array
		updateAutoCreateAppNameMap(resInfo, "delete", nameKey)

		if appResInfo.namespace == resInfo.namespace {
			if autoCreateAppMap != nil {
				maps := autoCreateAppMap[resInfo.namespace]
				values, _ := maps[nameKey]

				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Deleting application name/namespace: %s/%s", appResInfo.namespace, appResInfo.name))
				}

				if values == nil || len(values) == 0 {
					//delete app resource only when autoCreateName or part-of array is empty
					err := deleteResource(resController, &appResInfo.resourceInfo)
					if err != nil {
						if logger.IsEnabled(LogTypeError) {
							logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error deleting application: %s %s %s. Error: %s", resInfo.kind, resInfo.namespace, resInfo.name, err))
						}
					} else {
						if logger.IsEnabled(LogTypeInfo) {
							logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Deleted auto-created application %s/%s", appResInfo.namespace, appResInfo.name))
						}
					}
					return err
				}
			}
		}

		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("application %s/%s not auto-created", appResInfo.namespace, appResInfo.name))
		}
	}
	return nil
}

func autoCreateModifyApplication(resController *ClusterWatcher, rw *ResourceWatcher, resInfo *autoCreateResourceInfo, nameKey string) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("From %s/%s for auto-created: %s", resInfo.resourceInfo.namespace, resInfo.resourceInfo.name, nameKey))
	}
	gvr, ok := resController.getWatchGVR(coreApplicationGVR)
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

		unstructuredObj, err = intf.Get(nameKey, metav1.GetOptions{})
				
		// create app
		if err != nil {
			/* TODO: check error. Most likely resource does not exist */			
			err = createApplication(resController, rw, resInfo, nameKey)
			return err
		}

		/* If we get here, modify the applications */
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Checking existing application %s", unstructuredObj))
		}
		appResInfo := &appResourceInfo{}
		err = resController.parseAppResource(unstructuredObj, appResInfo)
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("parseApplication error %s", err))
			}
			return err
		}
		if !autoGenerated(appResInfo) {
			autoGenErr := fmt.Errorf("Unable to update application %s/%s as it was not auto-generated", resInfo.namespace, resInfo.name)
			if logger.IsEnabled(LogTypeError) {
				str := fmt.Sprintf("%s", autoGenErr)
				logger.Log(CallerName(), LogTypeError, str)
			}
			return autoGenErr
		}

		if autoCreatedApplicationNeedsUpdate(appResInfo, resInfo, nameKey) {
			// change status
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Changing autocreated application annotations and labels for %s/%s", resInfo.namespace, nameKey))
			}
			setApplicationAnnotationLabels(unstructuredObj, resInfo, nameKey)
			
			_, err = intf.Update(unstructuredObj, metav1.UpdateOptions{})
			if err != nil {
				if logger.IsEnabled(LogTypeError) {
					logger.Log(CallerName(), LogTypeError, fmt.Sprintf("    error modifying application%s", err))
				}
			} else {
				if logger.IsEnabled(LogTypeInfo) {
					logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Updated auto-created application: %s/%s", resInfo.namespace, nameKey))
				}
			}
			return err
		}
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, "Application does not need update")
		}
		return nil
	}
	return fmt.Errorf("Unable to find GVR for kind Application")
}

var autoCreatedAppJSONTemplate = "{ {{__NEWLINE__}}" +
	"    \"apiVersion\": \"app.k8s.io/v1beta1\", {{__NEWLINE__}}" +
	"    \"kind\": \"Application\", {{__NEWLINE__}}" +
	"    \"metadata\": { {{__NEWLINE__}}" +
	"        \"labels\": {{{__NEWLINE__}}" +
	"             \"kappnav.app.auto-created\": \"true\",{{__NEWLINE__}}" +
	"             \"app.kubernetes.io/name\": \"{{__APP_NAME__}}\", {{__NEWLINE__}}" +
	"             \"app.kubernetes.io/version\": \"{{__APP_VERSION__}}\", {{__NEWLINE__}}" +
	"             \"{{__APP_LABEL__}}\": \"{{__APP_NAME__}}\" {{__NEWLINE__}}" +
	"        },{{__NEWLINE__}}" +
	"        \"name\": \"{{__APP_NAME__}}\",{{__NEWLINE__}}" +
	"        \"namespace\": \"{{__NAMESPACE__}}\"{{__NEWLINE__}}" +
	"    },{{__NEWLINE__}}" +
	"    \"spec\": { {{__NEWLINE__}}" +
	"        \"componentKinds\": [ {{__COMPONENT_KINDS__}} ], {{__NEWLINE__}}" +
	"        \"selector\": { {{__SELECTOR_VALUE__}} } {{__NEWLINE__}}" +
	"        }{{__NEWLINE__}}" +
	"    }{{__NEWLINE__}}" +
	"}{{__NEWLINE__}}"

var autoCreatedAppJSONPartOfTemplate = "{ {{__NEWLINE__}}" +
	"    \"apiVersion\": \"app.k8s.io/v1beta1\", {{__NEWLINE__}}" +
	"    \"kind\": \"Application\", {{__NEWLINE__}}" +
	"    \"metadata\": { {{__NEWLINE__}}" +
	"        \"labels\": {{{__NEWLINE__}}" +
	"             \"kappnav.app.auto-created\": \"true\",{{__NEWLINE__}}" +
	"             \"app.kubernetes.io/name\": \"{{__APP_NAME__}}\", {{__NEWLINE__}}" +
	"             \"app.kubernetes.io/version\": \"{{__APP_VERSION__}}\", {{__NEWLINE__}}" +
	"             \"app.kubernetes.io/part-of\": \"{{__APP_NAME__}}\" {{__NEWLINE__}}" +
	"        },{{__NEWLINE__}}" +
	"        \"name\": \"{{__APP_NAME__}}\",{{__NEWLINE__}}" +
	"        \"namespace\": \"{{__NAMESPACE__}}\"{{__NEWLINE__}}" +
	"    },{{__NEWLINE__}}" +
	"    \"spec\": { {{__NEWLINE__}}" +
	"        \"componentKinds\": [ {{__COMPONENT_KINDS__}} ], {{__NEWLINE__}}" +
	"        \"selector\": { {{__SELECTOR_VALUE__}} } {{__NEWLINE__}}" +
	"        }{{__NEWLINE__}}" +
	"    }{{__NEWLINE__}}" +
	"}{{__NEWLINE__}}"

var autoCreateMatchLabel = "\"matchLabels\": { \"{{__APP_LABEL__}}\": \"{{__LABEL_VALUE__}}\"  } "
var autoCreateMatchLabelPartOf = "\"matchLabels\": { \"{{__PART_OF_LABEL__}}\": \"{{__LABEL_VALUE__}}\"  } "

var autoCreateMatchExpression = "\"matchExpressions\": [ { \"key\": \"{{__APP_LABEL__}}\",  \"operator\": \"In\", \"values\": [{{__MATCH_VALUES__}}] } ]}"
var autoCreateMatchExpressionPartOf = "\"matchExpressions\": [ { \"key\": \"{{__PART_OF_LABEL__}}\",  \"operator\": \"In\", \"values\": [{{__MATCH_VALUES__}}] } ]}"

func getApplicationJSON(resInfo *autoCreateResourceInfo, nameKey string) string {
	var template string

	if resInfo.partOfLabel == nameKey {
		template = strings.Replace(autoCreatedAppJSONPartOfTemplate, "{{__NEWLINE__}}", "\n", -1)
		template = strings.Replace(template, "{{__APP_NAME__}}", resInfo.partOfLabel, -1)
		template = strings.Replace(template, "{{__PART_OF_LABEL__}}", "app.kubernetes.io/part-of", -1)
	} else { // for autoCreateName
		template = strings.Replace(autoCreatedAppJSONTemplate, "{{__NEWLINE__}}", "\n", -1)
		template = strings.Replace(template, "{{__APP_NAME__}}", resInfo.autoCreateName, -1)
		template = strings.Replace(template, "{{__APP_LABEL__}}", resInfo.autoCreateLabel, -1)
	}

	template = strings.Replace(template, "{{__APP_VERSION__}}", resInfo.autoCreateVersion, -1)
	template = strings.Replace(template, "{{__NAMESPACE__}}", resInfo.namespace, -1)

	componentKindsStr := ""
	for index, componentKind := range resInfo.autoCreateKinds {
		componentKindsStr += "{ \"group\" : \"" + componentKind.group + "\", \"kind\" : \"" + componentKind.kind + "\" }"
		if index < len(resInfo.autoCreateKinds)-1 {
			componentKindsStr += ", "
		}
	}

	template = strings.Replace(template, "{{__COMPONENT_KINDS__}}", componentKindsStr, -1)

	var selectorValue string

	if len(resInfo.autoCreateLabelValues) == 1 {
		if nameKey == resInfo.autoCreateName {
			selectorValue = strings.Replace(autoCreateMatchLabel, "{{__NEWLINE__}}", "\n", -1)
			selectorValue = strings.Replace(selectorValue, "{{__NEWLINE__}}", "\n", -1)
			selectorValue = strings.Replace(selectorValue, "{{__APP_LABEL__}}", resInfo.autoCreateLabel, -1)
			selectorValue = strings.Replace(selectorValue, "{{__LABEL_VALUE__}}", resInfo.autoCreateLabelValues[0], -1)
		} else { //part-of
			selectorValue = strings.Replace(autoCreateMatchLabelPartOf, "{{__NEWLINE__}}", "\n", -1)
			selectorValue = strings.Replace(selectorValue, "{{__NEWLINE__}}", "\n", -1)
			selectorValue = strings.Replace(selectorValue, "{{__PART_OF_LABEL__}}", "app.kubernetes.io/part-of", -1)
			
			//if part-of label is different from original, use original value
			if resInfo.orgPartOfLabel != resInfo.partOfLabel {
				selectorValue = strings.Replace(selectorValue, "{{__LABEL_VALUE__}}", resInfo.orgPartOfLabel, -1)	
			} else {
				selectorValue = strings.Replace(selectorValue, "{{__LABEL_VALUE__}}", resInfo.partOfLabel, -1)	
			}
		} 
	} else {
		if nameKey == resInfo.autoCreateName {
			selectorValue = strings.Replace(autoCreateMatchExpression, "{{__NEWLINE__}}", "\n", -1)
			selectorValue = strings.Replace(selectorValue, "{{__APP_LABEL__}}", resInfo.autoCreateLabel, -1)
		} else { //part-of
			selectorValue = strings.Replace(autoCreateMatchExpressionPartOf, "{{__NEWLINE__}}", "\n", -1)
			selectorValue = strings.Replace(selectorValue, "{{__PART_OF_LABEL__}}", "app.kubernetes.io/part-of", -1)
		} 
		
		matchValuesStr := ""
		if nameKey == resInfo.autoCreateName {
			for index, value := range resInfo.autoCreateLabelValues {
				matchValuesStr += "\"" + value + "\""
				if index < len(resInfo.autoCreateLabelValues)-1 {
					matchValuesStr += ", "
				}
			}
		} else {
			matchValuesStr += resInfo.partOfLabel
		}
		selectorValue = strings.Replace(selectorValue, "{{__MATCH_VALUES__}}", matchValuesStr, -1)
	}

	template = strings.Replace(template, "{{__SELECTOR_VALUE__}}", selectorValue, -1)
	return template
}

/* Create application. Assume it does not already exist */
func createApplication(resController *ClusterWatcher, rw *ResourceWatcher, resInfo *autoCreateResourceInfo, nameKey string) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, "Creating application "+resInfo.namespace+"/"+nameKey+", from "+resInfo.name)
	}
	gvr, ok := resController.getWatchGVR(coreApplicationGVR)
	if ok {
		var intfNoNS = resController.plugin.dynamicClient.Resource(gvr)
		var intf dynamic.ResourceInterface
		if resInfo.namespace != "" {
			intf = intfNoNS.Namespace(resInfo.namespace)
		} else {
			intf = intfNoNS
		}

		// get application JSON template for autoCreateName or partOfLabel
		template := getApplicationJSON(resInfo, nameKey)
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("createApplication JSON: %s", template))
		}
		
		var unstructuredObj = &unstructured.Unstructured{}
		err := unstructuredObj.UnmarshalJSON([]byte(template))
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Unable to convert JSON %s to unstructured.  Error: %s", template, err))
			}
			return err
		}
		
		_, err = intf.Create(unstructuredObj, metav1.CreateOptions{})
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Unable to create Application %s/%s error: %s", resInfo.namespace, resInfo.autoCreateName, err))
			}
			return err
		}
	} else {
		err := fmt.Errorf("Unable to get GVR for Application")
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Unable to create Application %s/%s.  Error: %s", resInfo.namespace, resInfo.autoCreateName, err))
		}
		return err
	}
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("Created application %s/%s", resInfo.namespace, resInfo.autoCreateName))
	}
	return nil
}

func autoGenerated(appResInfo *appResourceInfo) bool {
	return appResInfo.labels[AppAutoCreated] == "true"
}

/* Set annotations and labels for an existing application
 */
func setApplicationAnnotationLabels(unstructuredObj *unstructured.Unstructured, resInfo *autoCreateResourceInfo, nameKey string) {
	var objMap = unstructuredObj.Object
	metadataObj, ok := objMap[METADATA]
	var metadata map[string]interface{}
	if !ok {
		metadata = make(map[string]interface{})
		objMap[METADATA] = metadata
	} else {
		metadata = metadataObj.(map[string]interface{})
	}

	var annotations map[string]interface{}
	annotationsObj, ok := metadata[ANNOTATIONS]
	if !ok {
		annotations = make(map[string]interface{})
		metadata[ANNOTATIONS] = annotations
	} else {
		annotations = annotationsObj.(map[string]interface{})
	}

	// set part-of label
	labels := make(map[string]interface{})
	metadata[LABELS] = labels

	if nameKey == resInfo.autoCreateName {
		labels[resInfo.autoCreateLabel] = resInfo.autoCreateName
		labels[labelName] = resInfo.autoCreateName
	} else {
		labels[AppPartOf] = resInfo.partOfLabel
		labels[labelName] = resInfo.partOfLabel
	}
	
	labels[labelVersion] = resInfo.autoCreateVersion
	labels[labelAutoCreate] = "true"

	var spec map[string]interface{}
	specObj, ok := objMap[SPEC]
	if !ok {
		spec = make(map[string]interface{})
		objMap[SPEC] = spec
	} else {
		spec = specObj.(map[string]interface{})
	}
	var selector map[string]interface{}
	selectorObj, ok := spec[SELECTOR]
	if !ok {
		selector = make(map[string]interface{})
		spec[SELECTOR] = selector		
	} else {
		selector = selectorObj.(map[string]interface{})
	}

	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("selector: %v", selector))
	}

	if len(resInfo.autoCreateLabelValues) == 1 || nameKey == resInfo.partOfLabel {
		matchLabels := make(map[string]interface{})
		selector[MATCHLABELS] = matchLabels	
		if nameKey == resInfo.autoCreateName { 
			if len(resInfo.autoCreateLabelValues) == 1 {
				matchLabels[resInfo.autoCreateLabel] = resInfo.autoCreateLabelValues[0]			
			}
		} else { //part-of
			//if part-of label is different from original, use original value
			if resInfo.orgPartOfLabel != resInfo.partOfLabel {
				matchLabels[AppPartOf] = resInfo.orgPartOfLabel
			} else {
				matchLabels[AppPartOf] = resInfo.partOfLabel
			}
		}
		
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("matchLabels: %v", matchLabels))
		}
		delete(selector, MATCHEXPRESSIONS)
	} else {
		delete(selector, MATCHLABELS)
		matchExpressions := make([]interface{}, 0)
		expression := make(map[string]interface{})
		matchExpressions = append(matchExpressions, expression)
		selector[MATCHEXPRESSIONS] = matchExpressions

		if nameKey == resInfo.autoCreateName {
			expression[KEY] = resInfo.autoCreateLabel
		} else {
			expression[KEY] = AppPartOf
		}

		expression[OPERATOR] = OperatorIn

		values := make([]interface{}, 0)
		if nameKey == resInfo.autoCreateName {
			for _, val := range resInfo.autoCreateLabelValues {
				values = append(values, val) 
			}
		} else {
			//if part-of label is different from original, use original value
			if resInfo.orgPartOfLabel != resInfo.partOfLabel {
				values = append(values, resInfo.orgPartOfLabel) 
			} else {
				values = append(values, resInfo.partOfLabel) 
			}
		} 
		expression[VALUES] = values
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("matchExpression: %v", expression))
		}
	}

	componentKinds := make([]interface{}, 0)
	for _, ck := range resInfo.autoCreateKinds {
		ckMap := make(map[string]interface{})
		ckMap[GROUP] = ck.group
		ckMap[KIND] = ck.kind
		componentKinds = append(componentKinds, ckMap)
	}
	spec[COMPONENTKINDS] = componentKinds
}

func sameComponentKinds(kinds1 []groupKind, kinds2 []groupKind) bool {
	if len(kinds1) != len(kinds2) {
		return false
	}
	for index, gk1 := range kinds1 {
		gk2 := kinds2[index]
		if (gk1.group != gk2.group) || (gk1.kind != gk2.kind) {
			return false
		}
	}
	return true
}

func sameStringArray(array1 []string, array2 []string) bool {
	if len(array1) != len(array2) {
		return false
	}
	for index, str1 := range array1 {
		str2 := array2[index]
		if str1 != str2 {
			return false
		}
	}
	return true
}

func stringArrayToString(array []string) string {
	ret := "["
	length := len(array)
	for index, str := range array {
		ret += "\"" + str + "\""
		if index < length-1 {
			ret += ", "
		}
	}
	ret += "]"
	return ret
}

func sameMatchExpressions(expressions1 []matchExpression, expressions2 []matchExpression) bool {
	if len(expressions1) != len(expressions2) {
		return false
	}

	for index, expr1 := range expressions1 {
		expr2 := expressions2[index]
		if expr1.key != expr2.key {
			return false
		}
		if expr1.operator != expr2.operator {
			return false
		}
		if !sameStringArray(expr1.values, expr2.values) {
			return false
		}
	}
	return true
}

func autoCreatedApplicationNeedsUpdate(appResInfo *appResourceInfo, resInfo *autoCreateResourceInfo, nameKey string) bool {

	/* Check if auto-create labels have changed */
	if nameKey == resInfo.autoCreateName {
		if appResInfo.labels[resInfo.autoCreateLabel] != resInfo.autoCreateName {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Labels mismatch. Label: %s, label value in app: %s label value in original resource: %s", resInfo.autoCreateLabel, appResInfo.labels[resInfo.autoCreateLabel], resInfo.autoCreateName))
			}
			return true
		}
	} else { //part-of
		if appResInfo.labels[AppPartOf] != resInfo.partOfLabel {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Labels mismatch. Label: %s, label value in app: %s , label value in original resource: %s", AppPartOf, appResInfo.labels[AppPartOf], resInfo.partOfLabel))
			}
			return true
		}
	} 

	if appResInfo.labels[labelVersion] != resInfo.autoCreateVersion {
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Version mismatch. version label %s in app: %s. Version in original resource: %s", labelVersion, appResInfo.labels[labelVersion], resInfo.autoCreateVersion))
		}
		return true
	}

	// check if component kinds have changed
	if !sameComponentKinds(appResInfo.componentKinds, resInfo.autoCreateKinds) {
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Component kinds mismatch. In app: %v In original resource: %v", appResInfo.componentKinds, resInfo.autoCreateKinds))
		}
		return true
	}

	/* check if selectors have changed */
	lenMatchLabels := len(appResInfo.matchLabels)
	lenMatchExpressions := len(appResInfo.matchExpressions)

	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Match labels and expressions: %v, %v", appResInfo.matchLabels, appResInfo.matchExpressions))
	}
	if lenMatchLabels > 0 && lenMatchExpressions > 0 {
		// shouldn't happen
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Created app contains both match labels and expressions"))
		}
		return true
	}
	if lenMatchLabels == 1 {
		if len(resInfo.autoCreateLabelValues) != 1 {
			// number of labels don't match
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Application has 1 match label value, but original resource has %d label values", len(resInfo.autoCreateLabelValues)))
			}
			return true
		}
		if nameKey == resInfo.autoCreateName {
			if appResInfo.matchLabels[resInfo.autoCreateLabel] != resInfo.autoCreateLabelValues[0] {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Match label values mismatch. Created app: %s: %s, original resource: %s", resInfo.autoCreateLabel, appResInfo.matchLabels[resInfo.autoCreateLabel], resInfo.autoCreateLabelValues[0]))
				}
				// labels don't match
				return true
			}
		} else { //part-of
			if appResInfo.matchLabels[AppPartOf] != resInfo.partOfLabel {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Match label values mismatch. Created app: %s: %s, original resource: %s", AppPartOf, appResInfo.matchLabels[AppPartOf], resInfo.partOfLabel))
				}
				// labels don't match
				return true
			}
		} 
	} else if lenMatchExpressions == 1 {
		matchExpression := appResInfo.matchExpressions[0]
		// key != "app" && key != "app.kubernetes.io/part-of"
		if matchExpression.key != AppPartOf && matchExpression.key != resInfo.autoCreateLabel {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Match expression key mismatch. Created app key: %s, original resource key: %s", matchExpression.key, resInfo.autoCreateLabel))
			}
			return true
		}

		if matchExpression.operator != OperatorIn {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Create app operation is not %s", OperatorIn))
			}
			return true
		}

		// autoCreateName
		if nameKey == resInfo.autoCreateName {
			if !sameStringArray(matchExpression.values, resInfo.autoCreateLabelValues) {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Mismatched expression values. Create app values: %s, original resource values: %s", stringArrayToString(matchExpression.values), stringArrayToString(resInfo.autoCreateLabelValues)))
				}
				return true
			}
		} else { //part-of
			var partOfArray []string
			partOfArray[0] = resInfo.partOfLabel
			if !sameStringArray(matchExpression.values, partOfArray) {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Mismatched expression values. Create app values: %s, original resource values: %s", stringArrayToString(matchExpression.values), partOfArray))
				}
				return true
			}
		} 
	} else {
		// shouldn't happen
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Neither match label nor match express is length 1"))
		}
		return true
	}

	return false
}

/* Delte auto-created applications  whose original resource no longer exists
 */
func deleteOrphanedAutoCreatedApplications(resController *ClusterWatcher) error {
	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, "Deleting orphaned auto-created application\n")
	}

	gvr, ok := resController.getWatchGVR(coreApplicationGVR)
	if !ok {
		err := fmt.Errorf("Unable to get GVR for Application")
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error in deleteOrphanedAutoCreatedApplications: %s", err))
		}
		return err
	}
	var intf = resController.plugin.dynamicClient.Resource(gvr)

	// fetch the current resource
	var unstructuredList *unstructured.UnstructuredList
	var err error
	unstructuredList, err = intf.List(metav1.ListOptions{LabelSelector: "kappnav.app.auto-created=true"})
	if err != nil {
		// TODO: check error code. Most likely resource does not exist
		return err
	}

	for _, unstructuredObj := range unstructuredList.Items {
		var appResInfo = &appResourceInfo{}
		err = resController.parseAppResource(&unstructuredObj, appResInfo)
		if err != nil {
			continue
		}
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    deleteOrphanedAutoCreatedApplications checking application: %s\n", appResInfo.name))
		}

		if autoCreateAppMap != nil {
			maps := autoCreateAppMap[appResInfo.namespace]
			values, _ := maps[appResInfo.name]
			// delete auto-create app if no resource exists in the map
			if values == nil || len(values) == 0 {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("    deleting application: %s/%s created\n", appResInfo.namespace, appResInfo.name))
				}
				err := deleteResource(resController, &appResInfo.resourceInfo)
				if err != nil {
					if logger.IsEnabled(LogTypeError) {
						logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error deleting orphaned application:  %s/%s. Error: %s\n", appResInfo.namespace, appResInfo.name, err))
					}
				} else {
					if logger.IsEnabled(LogTypeInfo) {
						logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Deleted orphaned auto-created application %s/%s\n", appResInfo.namespace, appResInfo.name))
					}
				}
			}
		}
	}

	return nil
}

/* containsName check if an array contains name string */
func containsName(array []string, name string) bool {
	if array != nil {
		for _, a := range array {
			if a == name {
				return true
			}
		}
	}
	return false
}

func removeCharacters(input string, characters string) string {
	filter := func(r rune) rune {
		if strings.IndexRune(characters, r) < 0 {
			return r
		}
		return -1
	}

	return strings.Map(filter, input)

}

// fetch kappnav CR and get autoCreateKinds from kappnav CR
func getAutoCreateKindsFromKappnav(resController *ClusterWatcher) []groupKind {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, "getAutoCreateKindsFromKappnav")
	}
	var autoCreateKinds []groupKind
	gvr := schema.GroupVersionResource{
		Group:    "kappnav.operator.kappnav.io",
		Version:  "v1",
		Resource: "kappnavs",
	}
	var intfNoNS = resController.plugin.dynamicClient.Resource(gvr)
	var intf dynamic.ResourceInterface
	// get interface from kappnav namespace
	intf = intfNoNS.Namespace(getkAppNavNamespace())
	// fetch the kappnav custom resource obj
	var unstructuredObj *unstructured.Unstructured
	var resName = "kappnav"
	unstructuredObj, _ = intf.Get(resName, metav1.GetOptions{})
	
	if unstructuredObj != nil {
		var objMap = unstructuredObj.Object
		if objMap != nil {
			tmp, _ := objMap[SPEC]
			spec := tmp.(map[string]interface{})
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("found kappnav CR spec %s ", spec))
			}
			tmp1, _ := spec["autoCreateKinds"]
			autoCreateKindsMap := tmp1.([]interface{})
			if len(autoCreateKindsMap) > 0 {
				autoCreateKinds = make([]groupKind, 0, len(autoCreateKindsMap))
				for _, autoCreateKind := range autoCreateKindsMap {
					if logger.IsEnabled(LogTypeDebug) {
						logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("found autoCreateKind %s ", autoCreateKind))
					}
					group := autoCreateKind.(map[string]interface{})["group"]
					kind := autoCreateKind.(map[string]interface{})["kind"]
					groupStr, gok := group.(string)
					kindStr, kok := kind.(string)
					if !gok {
						groupStr = ""
					}
					if !kok {
						kindStr = ""
					}
					autoCreateKinds = append(autoCreateKinds, groupKind{groupStr, kindStr, nil})
				}
			}
		}
	}
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("autoCreateKinds: %s ", autoCreateKinds))
	}
	return autoCreateKinds
}

/* updateAutoCreateAppNameMap updates autoCreateAppMap when a resource (i.e. deployment) is added or deleted in a namespace */
func updateAutoCreateAppNameMap(resourceInfo *autoCreateResourceInfo, operation string, nameKey string) {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, "updateAutoCreateAppNameMap operation: "+operation)
	}

	if resourceInfo != nil {
		nameObj := resourceInfo.metadata[NAME]
		namespaceObj := resourceInfo.metadata[NAMESPACE]

		if nameObj != nil && namespaceObj != nil {
			name := nameObj.(string)
			namespace := namespaceObj.(string)

			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, "app name: "+name)
				logger.Log(CallerName(), LogTypeDebug, "app namespace: "+namespace)
				logger.Log(CallerName(), LogTypeDebug, "app auto-create name: "+resourceInfo.autoCreateName)
				logger.Log(CallerName(), LogTypeDebug, "app part-of label: "+resourceInfo.partOfLabel)
				logger.Log(CallerName(), LogTypeDebug, "nameKey: "+nameKey)
			}

			// key is namespace
			maps, ok := autoCreateAppMap[namespace]

			//an app is added or updated
			if operation == "add" {
				if !ok {
					if logger.IsEnabled(LogTypeDebug) {
						logger.Log(CallerName(), LogTypeDebug, "autoCreateName or partOfLabel does not exist - add it to map"+name)
					}
					// namespace does not exist, create a new map
					m := make(map[string][]string)
					m[nameKey] = []string{name}
					autoCreateAppMap[namespace] = m
				} else {
					//add app name to map if it does not exist
					values, ok := maps[nameKey]
					if !ok {
						// no partOfLabel entry
						maps[nameKey] = []string{name}
						autoCreateAppMap[namespace] = maps
					} else {
						if !containsName(values, name) {
							if logger.IsEnabled(LogTypeDebug) {
								logger.Log(CallerName(), LogTypeDebug, "autoCreateName or partOfLabel entry exists - add app name to map: "+name)
							}
							values = append(values, name)
							maps[nameKey] = values
							autoCreateAppMap[namespace] = maps
						}
					}
				}
			} else { //delete operation
				//an app is deleted
				if ok && len(maps) > 0 {
					values, _ := maps[nameKey]
					if logger.IsEnabled(LogTypeDebug) {
						logger.Log(CallerName(), LogTypeDebug, "delete app from array "+name)
					}
					for i, n := range values {
						if name == n {
							values = append(values[:i], values[i+1:]...)
							maps[nameKey] = values
							autoCreateAppMap[namespace] = maps
							break
						}
					}
					//delete autoCreateName or partOfLabel entry from maps if no any app exists
					if len(values) == 0 {
						delete(maps, nameKey)
					}
				}

				// delete namespace entry if it is empty
				if len(maps) == 0 {
					delete(autoCreateAppMap, namespace)
				}
			}

		}
	}

	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("autoCreateAppMap: %s ", autoCreateAppMap))
	}
}
