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
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	// KAMAutoCreate ...
	KAMAutoCreate = "kappnav.io/map-type"
	// KAMAutoCreated ...
	KAMAutoCreated = "kappnav.kam.auto-created"

	configMap = "ConfigMap"
	actions   = "actions"
	status    = "status"
	sections  = "sections"
	data      = "data"
	kamdefs   = "kam-defs"
	kamTrue   = "true"
	labels    = "labels"
)

var (
	kindActionMappingGVR = schema.GroupVersionResource{
		Group:    "actions.kappnav.io",
		Version:  "v1",
		Resource: "kindactionmappings",
	}

	/* group and kind for auto-created kam */
	kamGroupKind = groupKind{group: "actions.kappnav.io", kind: "KindActionMapping"}

	/* map to track kam name being added or deleted for an auto creation name */
	autoCreateKAMMap = make(map[string]map[string]string)
)

/* information about resource from which to auto-create kams */
type autoCreateKAMInfo struct {
	resourceInfo
	autoCreateKAMLabel     string
	autoCreatKAMApiVersion string
	autoCreateKAMKind      string
	autoCreateKAMName      string
	autoCreateKAMJson      string
	autoCreateKAMInfo      interface{}
}

/* Parse a configmap from which to auto create kams
   unstructuredObj: object to be parsed
   Return an autoCreateKAMInfo  to represent the resource
          Return  nil if the label kappnav.kam.auto-create is not set to actions/status/sections.
*/
func (resController *ClusterWatcher) parseConfigMapInfo(unstructuredObj *unstructured.Unstructured) *autoCreateKAMInfo {
	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug,
			fmt.Sprintf("Parse a configmap and check to see if it hosts a kam"))
	}

	resourceInfo := &autoCreateKAMInfo{}
	resController.parseResource(unstructuredObj, &resourceInfo.resourceInfo)

	// Only watch "ConfigMap" CR
	if resourceInfo.kind != configMap {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Not a ConfigMap kind: %s", resourceInfo.kind))
		}
		return nil
	}

	// check for the kam auto create label
	autoCreateLabel, ok := resourceInfo.resourceInfo.labels[KAMAutoCreate]
	if !ok {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("The kam auto create label does not exist in configmap %s", resourceInfo.name))
		}
		return nil
	}

	if autoCreateLabel != actions && autoCreateLabel != status && autoCreateLabel != sections {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("kam auto creation label value is not valid: %s", autoCreateLabel))
		}
		return nil
	}

	//parse the embedded kam resource info
	var objMap = unstructuredObj.Object
	dataMap, ok := objMap[data].(map[string]interface{})
	if !ok {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Configmap %s does not not contain \"data\" property", resourceInfo.name))
		}
		return nil
	}

	kamDefObj, ok := dataMap[kamdefs]
	if !ok {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Configmap %s does not not contain \"kam-defs\" property", resourceInfo.name))
		}
		return nil
	}

	resourceInfo.autoCreateKAMJson = kamDefObj.(string)

	kamDef, err := jsonToMap(resourceInfo.autoCreateKAMJson)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError,
				fmt.Sprintf("The value of \"kam-defs\" property contains invalid JSON in configmap %s with parsing error %s", resourceInfo.name, err))
		}
		return nil
	}
	parseKAMResource(kamDef, resourceInfo, resourceInfo.name)
	return resourceInfo
}

/* Json string to map conversion
 */
func jsonToMap(str string) (map[string]interface{}, error) {
	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug,
			fmt.Sprintf("The string value of \"kam-defs\" property is %s", str))
	}
	bytes := []byte(str)
	var kamMap map[string]interface{}
	err := json.Unmarshal(bytes, &kamMap)
	if err != nil {
		return nil, err
	}
	return kamMap, nil
}

/* parseKAMResource extracts selected KAAM resource fields to a structure
 */
func parseKAMResource(resource interface{}, kamResourceInfo *autoCreateKAMInfo, cName string) error {
	var objMap = resource.(map[string]interface{})
	var ok bool

	kamResourceInfo.autoCreatKAMApiVersion, ok = objMap[APIVERSION].(string)
	if !ok {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Configmap %s does not not contain \"kam-defs->apiVersion\" property", cName))
		}
		return nil
	}

	kamResourceInfo.autoCreateKAMKind, ok = objMap[KIND].(string)
	if !ok {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Configmap %s does not not contain \"kam-defs->kind\" property", cName))
		}
		return nil
	}

	metadataObj, ok := objMap[METADATA]
	if !ok {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Configmap %s does not not contain \"kam-defs->metadata\" property", cName))
		}
		return nil
	}

	var metadata map[string]interface{}
	metadata, ok = metadataObj.(map[string]interface{})
	kamResourceInfo.autoCreateKAMName, ok = metadata[NAME].(string)
	if !ok {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Configmap %s does not not contain \"kam-defs->metadata->name\" property", cName))
		}
	}
	return nil
}

/* Handle auto creation of kam from config map resources
 */
var autoCreateKAMHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {
	key := eventData.key
	_, exists, err := rw.store.GetByKey(key)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Fetching key %s from store failed: %v", key, err))
		}
		return err
	}

	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug,
			fmt.Sprintf("Processing %d current object: %s", eventData.funcType, eventData.obj.(*unstructured.Unstructured)))
	}

	if eventData.funcType == UpdateFunc {
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug,
				fmt.Sprintf("Processing %d old object : %s", eventData.funcType, eventData.oldObj.(*unstructured.Unstructured)))
		}
	}

	resourceInfo := resController.parseConfigMapInfo(eventData.obj.(*unstructured.Unstructured))
	if resourceInfo == nil {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo,
				fmt.Sprintf("Not a configmap or not a configmap that hosts a kam"))
		}
		return nil
	}

	// perform add/update/delete a kam operation
	if !exists { // hosting configmap is gone
		// KAM deleted
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug,
				fmt.Sprintf("Processing delete KAM %s for a deleted configmap %s", resourceInfo.autoCreateKAMName, key))
		}
		// Delete the auto-created kam
		deleteAutoCreatedKAMForConfigmap(resController, resourceInfo)
	} else if eventData.funcType == UpdateFunc || eventData.funcType == AddFunc {
		if eventData.funcType == UpdateFunc {
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug,
					fmt.Sprintf("Processing update KAM %s in configmap: %s", resourceInfo.autoCreateKAMName, key))
			}
			oldResInfo := resController.parseConfigMapInfo(eventData.oldObj.(*unstructured.Unstructured))
			if oldResInfo != nil {
				if oldResInfo.autoCreateKAMName != resourceInfo.autoCreateKAMName {
					if logger.IsEnabled(LogTypeInfo) {
						logger.Log(CallerName(), LogTypeInfo,
							fmt.Sprintf("Auto-created KAM for configmap %s changed from %s to %s. Deleting existing auto-created KAM",
								key, oldResInfo.autoCreateKAMName, resourceInfo.autoCreateKAMName))
					}
					// name of auto created resource changed. Deleted the old
					deleteAutoCreatedKAMForConfigmap(resController, oldResInfo)
				}
				// Update autoCreateName entry in case resource is changed
				updateAutoCreateKAMNameMap(resourceInfo, "add")
			}
		} else { // AddFunc
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug,
					fmt.Sprintf("Processing add KAM %s in configmap: %s", resourceInfo.autoCreateKAMName, key))
			}
			// a kam is added and add it to autoCreateKAMName array
			updateAutoCreateKAMNameMap(resourceInfo, "add")
		}

		err := autoCreateModifyKAM(resController, rw, resourceInfo)
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError,
					fmt.Sprintf("Error processing processig add/update KAM %s in configmap %s: %s", resourceInfo.autoCreateKAMName, key, err))
			}
			return err
		}
	} else {
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, "Skipping event")
		}
	}
	return nil
}

/* helper funtion to get a CRD client
 */
func getCRDClient(resController *ClusterWatcher, gvr schema.GroupVersionResource, resInfo *autoCreateKAMInfo) dynamic.ResourceInterface {
	var intfNoNS = resController.plugin.dynamicClient.Resource(gvr)
	var intf dynamic.ResourceInterface
	if resInfo.namespace != "" {
		intf = intfNoNS.Namespace(resInfo.namespace)
	} else {
		intf = intfNoNS
	}
	return intf
}

/* delete auto created kam
 */
func deleteAutoCreatedKAMForConfigmap(resController *ClusterWatcher, resInfo *autoCreateKAMInfo) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("Deleting auto-created KAM %s/%s", resInfo.namespace, resInfo.autoCreateKAMName))
	}

	gvr, ok := resController.getWatchGVR(kindActionMappingGVR)
	if ok {
		var intf dynamic.ResourceInterface = getCRDClient(resController, gvr, resInfo)

		// fetch the current kam resource
		var unstructuredObj *unstructured.Unstructured
		var err error
		unstructuredObj, err = intf.Get(resInfo.autoCreateKAMName, metav1.GetOptions{})
		if err != nil {
			/* Most likely resource does not exist */
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError,
					fmt.Sprintf("Error getting kam to be deleted %s/%s. Error:%s", resInfo.namespace, resInfo.autoCreateKAMName, err))
			}
			return nil
		}

		var kamResInfo = &resourceInfo{}
		resController.parseResource(unstructuredObj, kamResInfo)

		//delete kam name from autoCreateKAMName map
		updateAutoCreateKAMNameMap(resInfo, "delete")

		if kamResInfo.namespace == resInfo.namespace {
			if autoCreateKAMMap != nil {
				maps := autoCreateKAMMap[resInfo.namespace]
				exists, _ := maps[resInfo.autoCreateKAMName]

				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug,
						fmt.Sprintf("Deleting KAM name/namespace: %s/%s, exists = %s", kamResInfo.namespace, kamResInfo.name, exists))
				}

				if exists != kamTrue {
					//delete kam resource only when the entry of autoCreateKAMName does not exist
					err := deleteResource(resController, kamResInfo)
					if err != nil {
						if logger.IsEnabled(LogTypeError) {
							logger.Log(CallerName(), LogTypeError,
								fmt.Sprintf("Error deleting kam: %s/%s for configmap %s. Error: %s",
									resInfo.namespace, resInfo.autoCreateKAMName, resInfo.name, err))
						}
					} else {
						if logger.IsEnabled(LogTypeInfo) {
							logger.Log(CallerName(), LogTypeInfo,
								fmt.Sprintf("Deleted auto-created kam %s/%s", kamResInfo.namespace, kamResInfo.name))
						}
					}
					return err
				}
			}
		}

		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("kam %s/%s is deleted and not auto-created", kamResInfo.namespace, kamResInfo.name))
		}
	}

	return nil
}

/* auto create/update kam
 */
func autoCreateModifyKAM(resController *ClusterWatcher, rw *ResourceWatcher, resInfo *autoCreateKAMInfo) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry,
			fmt.Sprintf("Creating kam %s/%s for configmap %s", resInfo.resourceInfo.namespace, resInfo.autoCreateKAMName, resInfo.name))
	}
	gvr, ok := resController.getWatchGVR(kindActionMappingGVR)
	if ok {
		var intf dynamic.ResourceInterface = getCRDClient(resController, gvr, resInfo)

		// fetch the current kam resource
		var unstructuredObj *unstructured.Unstructured
		var err error
		unstructuredObj, err = intf.Get(resInfo.autoCreateKAMName, metav1.GetOptions{})
		if err != nil { // add kam
			/* TODO: check error. Most likely kam resource does not exist */
			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("createKAM JSON: %s", resInfo.autoCreateKAMJson))
			}

			unstructuredObj = &unstructured.Unstructured{}
			err := unstructuredObj.UnmarshalJSON([]byte(resInfo.autoCreateKAMJson))
			if err != nil {
				if logger.IsEnabled(LogTypeError) {
					logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Unable to convert JSON %s to unstructured", resInfo.autoCreateKAMJson))
				}
				return err
			}
			setKAMCreatedLabel(unstructuredObj)
			_, err = intf.Create(unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				if logger.IsEnabled(LogTypeError) {
					logger.Log(CallerName(), LogTypeError,
						fmt.Sprintf("Unable to create KAM %s/%s error: %s", resInfo.namespace, resInfo.autoCreateKAMName, err))
				}
				return err
			}
			return err
		}

		/* If we get here, modify the kam */
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Checking existing kam %s", unstructuredObj))
		}

		// Check to see if the existing kam is auto-created
		kamResInfo := &resourceInfo{}
		resController.parseResource(unstructuredObj, kamResInfo)
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("parseResource error %s", err))
			}
			return err
		}

		if !kamAutoGenerated(kamResInfo) {
			autoGenErr := fmt.Errorf("Unable to update kam %s/%s as it was not auto-generated", resInfo.namespace, resInfo.name)
			if logger.IsEnabled(LogTypeError) {
				str := fmt.Sprintf("%s", autoGenErr)
				logger.Log(CallerName(), LogTypeError, str)
			}
			return autoGenErr
		}

		// Replace the existing kam with the new kam
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("updateKAM JSON: %s", resInfo.autoCreateKAMJson))
		}

		unstructuredObjForUpdate := &unstructured.Unstructured{}
		err = unstructuredObjForUpdate.UnmarshalJSON([]byte(resInfo.autoCreateKAMJson))
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Unable to convert JSON %s to unstructured", resInfo.autoCreateKAMJson))
			}
			return err
		}

		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("Replacing auto created kam for %s/%s", resInfo.name, resInfo.namespace))
		}

		setMetadataForUpdate(unstructuredObj, unstructuredObjForUpdate)
		_, err = intf.Update(unstructuredObjForUpdate, metav1.UpdateOptions{})
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("error modifying kam %s", err))
			}
		} else {
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("Updated auto-created kam %s/%s", resInfo.namespace, resInfo.name))
			}
		}
		return err
	}

	err := fmt.Errorf("Unable to get GVR for KAM")
	if logger.IsEnabled(LogTypeError) {
		logger.Log(CallerName(), LogTypeError,
			fmt.Sprintf("Unable to create KAM %s/%s.  Error: %s", resInfo.namespace, resInfo.autoCreateKAMName, err))
	}
	return err
}

/* Set metadate for the kam to be updated
 */
func setMetadataForUpdate(existingObj *unstructured.Unstructured, objForUpdate *unstructured.Unstructured) {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("setMetadataForUpdate"))
	}
	var objMap = existingObj.Object
	metadataObj, ok := objMap[METADATA]
	var metadata map[string]interface{}
	if !ok {
		metadata = make(map[string]interface{})
		objMap[METADATA] = metadata
	} else {
		metadata = metadataObj.(map[string]interface{})
	}

	var objMapForUpdate = objForUpdate.Object
	objMapForUpdate[METADATA] = metadata

	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("setMetadataForUpdate: update metadata %s is set", metadata))
	}
}

/* Set labels for an existing kam
 */
func setKAMCreatedLabel(unstructuredObj *unstructured.Unstructured) {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, fmt.Sprintf("SetKAMCreatedLabel"))
	}
	var objMap = unstructuredObj.Object
	metadataObj, ok := objMap[METADATA]
	var metadata map[string]interface{}
	if !ok {
		metadata = make(map[string]interface{})
		objMap[METADATA] = metadata
	} else {
		metadata = metadataObj.(map[string]interface{})
	}

	labels := make(map[string]interface{})
	labels[KAMAutoCreated] = "true"
	metadata[LABELS] = labels

	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("SetKAMCreatedLabel: label %s is set", labels))
	}
}

/* Check if the kam is auto created
 */
func kamAutoGenerated(kamResInfo *resourceInfo) bool {
	return kamResInfo.labels[KAMAutoCreated] == "true"
}

/* Delte auto-created KAMs whose host configmap no longer exists
 */
func deleteOrphanedAutoCreatedKAMs(resController *ClusterWatcher) error {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, "Delete orphaned auto-created KAMs")
	}

	gvr, ok := resController.getWatchGVR(kindActionMappingGVR)
	if !ok {
		err := fmt.Errorf("Unable to get GVR for KAM")
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("Error in deleteOrphanedAutoCreatedKAMs: %s", err))
		}
		return err
	}
	var intf = resController.plugin.dynamicClient.Resource(gvr)

	// fetch the current resource
	var unstructuredList *unstructured.UnstructuredList
	var err error
	unstructuredList, err = intf.List(metav1.ListOptions{LabelSelector: "kappnav.kam.auto-created=true"})
	if err != nil {
		// TODO: check error code. Most likely resource does not exist
		return err
	}

	for _, unstructuredObj := range unstructuredList.Items {
		var kamResInfo = &resourceInfo{}
		resController.parseResource(&unstructuredObj, kamResInfo)
		if logger.IsEnabled(LogTypeDebug) {
			logger.Log(CallerName(), LogTypeDebug,
				fmt.Sprintf("DeleteOrphanedAutoCreatedKAMs kam: %s", kamResInfo.name))
		}

		if autoCreateKAMMap != nil {
			maps := autoCreateKAMMap[kamResInfo.namespace]
			exists, _ := maps[kamResInfo.name]

			// delete auto-create kam if no resource exists in the map
			if exists != kamTrue {
				if logger.IsEnabled(LogTypeDebug) {
					logger.Log(CallerName(), LogTypeDebug,
						fmt.Sprintf("Deleting the auto-created KAM: %s/%s", kamResInfo.namespace, kamResInfo.name))
				}
				err := deleteResource(resController, kamResInfo)
				if err != nil {
					if logger.IsEnabled(LogTypeError) {
						logger.Log(CallerName(), LogTypeError,
							fmt.Sprintf("Error deleting orphaned kam:  %s/%s. Error: %s", kamResInfo.namespace, kamResInfo.name, err))
					}
				} else {
					if logger.IsEnabled(LogTypeInfo) {
						logger.Log(CallerName(), LogTypeInfo,
							fmt.Sprintf("Deleted orphaned auto-created kam %s/%s", kamResInfo.namespace, kamResInfo.name))
					}
				}
			}
		}
	}

	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, "Delete orphaned auto-created KAMs")
	}
	return nil
}

/* updateAutoCreateKAMNameMap updates autoCreateKAMMap when a configmap is added or deleted in a namespace
 */
func updateAutoCreateKAMNameMap(resourceInfo *autoCreateKAMInfo, operation string) {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, "updateAutoCreateKAMNameMap operation: "+operation)
	}

	if resourceInfo != nil {
		name := resourceInfo.autoCreateKAMName
		cMapNameObj := resourceInfo.metadata[NAME]
		namespaceObj := resourceInfo.metadata[NAMESPACE]

		if cMapNameObj != nil && namespaceObj != nil {
			cMapName := cMapNameObj.(string)
			namespace := namespaceObj.(string)

			if logger.IsEnabled(LogTypeDebug) {
				logger.Log(CallerName(), LogTypeDebug, "name of configmap hosting the kam: "+cMapName)
				logger.Log(CallerName(), LogTypeDebug, "kam namespace: "+namespace)
				logger.Log(CallerName(), LogTypeDebug, "kam auto-create name: "+name)
			}

			// key is namespace
			kamMap, ok := autoCreateKAMMap[namespace]

			// a kam is added or updated
			if operation == "add" {
				if !ok {
					if logger.IsEnabled(LogTypeDebug) {
						logger.Log(CallerName(), LogTypeDebug, "autoCreateKAMName does not exist - add it to map "+name)
					}
					// namespace does not exist, create a new map
					km := make(map[string]string)
					km[name] = kamTrue
					autoCreateKAMMap[namespace] = km
				} else {
					// add kam name to map if it does not exist
					_, ok := kamMap[name]
					if !ok {
						// no autoCreateKAMName entry
						if logger.IsEnabled(LogTypeDebug) {
							logger.Log(CallerName(), LogTypeDebug, "autoCreateKAMName entry exists - add kam name to map: "+name)
						}
						kamMap[name] = kamTrue
						autoCreateKAMMap[namespace] = kamMap
					}
				}
			} else { // delete kam
				// delete the kam entry from the map key by namespace
				if ok && len(kamMap) > 0 {
					// delete autoCreateKAMName entry from maps
					exists, _ := kamMap[name]
					if exists == kamTrue {
						delete(kamMap, name)
					}
				}

				// delete namespace entry if it is empty
				if len(kamMap) == 0 {
					delete(autoCreateKAMMap, namespace)
				}
			}
		}
	}

	if logger.IsEnabled(LogTypeDebug) {
		logger.Log(CallerName(), LogTypeDebug, fmt.Sprintf("autoCreateKAMMap: %s ", autoCreateKAMMap))
	}
}
