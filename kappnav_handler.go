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
	"gopkg.in/yaml.v2"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"
	"os"
)

var (
	configMapV1Client    corev1.ConfigMapInterface
	featuredAppScriptURL string
	appLauncherScriptURL string
	iconScriptURL        string
	routeHost            string
)

// KappnavResourceInfo contains info from a KAppNav resource
type KappnavResourceInfo struct {
	okdFeaturedApp string
	okdAppLauncher string
}

// KAppNavHandler processes changes to the KAppNav custom resource
var KAppNavHandler resourceActionFunc = func(resController *ClusterWatcher, rw *ResourceWatcher, eventData *eventHandlerData) error {

	klog.Info("KAppNavHandler called for kind KAppNav")
	key := eventData.key
	_, exists, err := rw.store.GetByKey(key)
	if err != nil {
		klog.Errorf("fetching key %s failed: %v", key, err)
		return err
	}
	if !exists {
		// KAppNav kind has been deleted
		klog.Info("KAppNav kind has been deleted")
		resController.deleteGVR(eventData.obj)
	} else {
		// KAppNav resource added, modified, deleted
		if eventData.funcType == AddFunc {
			klog.Info("Event type: add")
			addKappnav(eventData)
		} else if eventData.funcType == UpdateFunc {
			klog.Info("Event type: update")
			updateKappnav(eventData)
		} else if eventData.funcType == DeleteFunc {
			klog.Info("Event type: delete")
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
	klog.Info("KUBE_ENV = " + kubeEnv)
	if kubeEnv == "okd" {

		if isLatestOKD {
			// TODO: Handle Latest/OpenShiftWebConsoleConfig case
			//cl, err := client.New(cfg, client.Options{})
		} else {
			update := false
			var kappnavInfo = &KappnavResourceInfo{}
			parseKAppNavResource(eventData.obj, kappnavInfo)
			getKappnavWebConsoleExtensionURLs()
			scriptURLs := getCurrentWebConsoleExtensionURLs("scriptURLs").([]interface{})
			klog.Infof("scriptURLs = %v", scriptURLs)

			if scriptURLs != nil {
				// Featured App scriptURL
				if kappnavInfo.okdFeaturedApp == "enabled" {
					if !containsString(scriptURLs, featuredAppScriptURL) {
						update = true
						scriptURLs = addString(scriptURLs, featuredAppScriptURL)
					}
				} else if kappnavInfo.okdFeaturedApp == "disabled" {
					if containsString(scriptURLs, featuredAppScriptURL) {
						update = true
						scriptURLs = removeString(scriptURLs, featuredAppScriptURL)
					}
				} else if kappnavInfo.okdFeaturedApp != "" {
					klog.Infof("Invalid value for KAppNav: console.okdFeaturedApp = %v", kappnavInfo.okdFeaturedApp)
				}

				// App Launcher scriptURL
				klog.Infof("Current scriptURLs: %v", scriptURLs)
				if kappnavInfo.okdAppLauncher == "enabled" {
					if !containsString(scriptURLs, appLauncherScriptURL) {
						update = true
						scriptURLs = addString(scriptURLs, appLauncherScriptURL)
					}
				} else if kappnavInfo.okdAppLauncher == "disabled" {
					if containsString(scriptURLs, appLauncherScriptURL) {
						update = true
						scriptURLs = removeString(scriptURLs, appLauncherScriptURL)
					}
				} else if kappnavInfo.okdAppLauncher != "" {
					klog.Infof("Invalid value for KAppNav: console.okdAppLauncher = %v", kappnavInfo.okdAppLauncher)
				}
			}

			// Icon css stylesheetURL
			stylesheetURLs := getCurrentWebConsoleExtensionURLs("stylesheetURLs").([]interface{})
			klog.Infof("stylesheetURLs = %v", stylesheetURLs)
			if stylesheetURLs != nil {
				if kappnavInfo.okdAppLauncher == "enabled" ||
					kappnavInfo.okdFeaturedApp == "enabled" {
					if !containsString(stylesheetURLs, iconScriptURL) {
						update = true
						stylesheetURLs = addString(stylesheetURLs, iconScriptURL)
					}
				} else if containsString(stylesheetURLs, iconScriptURL) {
					update = true
					stylesheetURLs = removeString(stylesheetURLs, iconScriptURL)
				}
			}
			if update {
				wcc := getWebConsoleConfigYaml()
				if wcc != nil {
					wcc["extensions"].(map[interface{}]interface{})["scriptURLs"] = scriptURLs
					wcc["extensions"].(map[interface{}]interface{})["stylesheetURLs"] = stylesheetURLs
					updateWebConsoleConfig(wcc)
				}
			}
		}
	}
	return nil
}

// deleteKappnav is called when the KAppNav resource is deleted
func deleteKappnav(eventData *eventHandlerData) error {
	kubeEnv := os.Getenv("KUBE_ENV")
	klog.Info("KUBE_ENV = " + kubeEnv)
	if kubeEnv == "okd" {

		if isLatestOKD {
			// TODO: Handle Latest/OpenShiftWebConsoleConfig case
			//cl, err := client.New(cfg, client.Options{})
		} else {
			update := false
			var kappnavInfo = &KappnavResourceInfo{}
			parseKAppNavResource(eventData.obj, kappnavInfo)
			getKappnavWebConsoleExtensionURLs()
			scriptURLs := getCurrentWebConsoleExtensionURLs("scriptURLs").([]interface{})
			klog.Infof("scriptURLs = %v", scriptURLs)
			if scriptURLs != nil {
				// remove Featured App scriptURL
				if containsString(scriptURLs, featuredAppScriptURL) {
					update = true
					scriptURLs = removeString(scriptURLs, featuredAppScriptURL)
				}

				// remove App Launcher scriptURL
				klog.Infof("Current scriptURLs: %v", scriptURLs)
				if scriptURLs != nil && containsString(scriptURLs, appLauncherScriptURL) {
					update = true
					scriptURLs = removeString(scriptURLs, appLauncherScriptURL)
				}
			}
			// remove stylesheetURL
			stylesheetURLs := getCurrentWebConsoleExtensionURLs("stylesheetURLs").([]interface{})
			klog.Infof("stylesheetURLs = %v", stylesheetURLs)
			if stylesheetURLs != nil && containsString(stylesheetURLs, iconScriptURL) {
				update = true
				stylesheetURLs = removeString(stylesheetURLs, iconScriptURL)
			}
			if update {
				wcc := getWebConsoleConfigYaml()
				if wcc != nil {
					wcc["extensions"].(map[interface{}]interface{})["scriptURLs"] = scriptURLs
					wcc["extensions"].(map[interface{}]interface{})["stylesheetURLs"] = stylesheetURLs
					updateWebConsoleConfig(wcc)
				}
			}
		}
	}
	return nil
}

// getCurrentWebConsoleExtensionURLs returns the URLs for the given
// extension currently in the webconsole-config ConfigMap
func getCurrentWebConsoleExtensionURLs(extension string) interface{} {
	wcc := getWebConsoleConfigYaml()
	if wcc != nil {
		return wcc["extensions"].(map[interface{}]interface{})[extension]
	}
	return nil
}

// setWebConsoleExtensionURLs sets the given extension URLs to
// the provided array of URLs in the webconsole-config ConfigMap
func setWebConsoleExtensionURLs(wcc map[interface{}]interface{}, extension string, urls []string) {
	wcc["extensions"].(map[interface{}]interface{})[extension] = urls
}

// getWebConsoleConfigYaml returns the webconsole-config.yaml
// data field from the webconsole-config ConfigMap
func getWebConsoleConfigYaml() map[interface{}]interface{} {
	var wcc map[interface{}]interface{}
	webConsoleConfigMap, err := getConfigMapV1Client().Get("webconsole-config", apismetav1.GetOptions{})
	if err == nil && webConsoleConfigMap != nil {
		wccString := webConsoleConfigMap.Data["webconsole-config.yaml"]
		klog.Info("webconsole-config.yaml = \n\n" + wccString + "\n")
		wcc = make(map[interface{}]interface{})
		err := yaml.Unmarshal([]byte(wccString), &wcc)
		if err != nil {
			klog.Errorf("error: %v", err)
		}
	} else {
		klog.Errorf("error: %v", err)
	}
	return wcc
}

func updateWebConsoleConfig(wcc map[interface{}]interface{}) {
	d, err1 := yaml.Marshal(&wcc)
	if err1 != nil {
		klog.Infof("error1: %v", err1)
	}
	klog.Infof("Updated webconsole-config.yaml:\n%s\n\n", string(d))
	webConsoleConfigMap, _ := getConfigMapV1Client().Get("webconsole-config", apismetav1.GetOptions{})
	webConsoleConfigMap.Data["webconsole-config.yaml"] = string(d)
	_, err := getConfigMapV1Client().Update(webConsoleConfigMap)
	if err != nil {
		klog.Errorf("Error updating web console ConfigMap: %v", err)
	}
}

func getConfigMapV1Client() corev1.ConfigMapInterface {
	if configMapV1Client == nil {
		configMapV1Client = kubeClient.CoreV1().ConfigMaps(OpenShiftWebConsole)
	}
	return configMapV1Client
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
	tmp1, _ := spec["console"]
	console := tmp1.(map[string]interface{})
	kappnavResource.okdAppLauncher = console["okdAppLauncher"].(string)
	klog.Infof("KAppNav resource spec.console.okdAppLauncher: %v", kappnavResource.okdAppLauncher)
	kappnavResource.okdFeaturedApp = console["okdFeaturedApp"].(string)
	klog.Infof("KAppNav resource spec.console.okdFeaturedApp: %v", kappnavResource.okdFeaturedApp)
	return nil
}

// getKappnavWebConsoleExtensionURLs calculates URLs
// needed to enable the KAppNav console integration
func getKappnavWebConsoleExtensionURLs() {
	if routeHost == "" && routeV1Client != nil {
		route, _ := routeV1Client.Routes(getkAppNavNamespace()).Get(KappnavUIService, apismetav1.GetOptions{})
		routeHost := route.Spec.Host
		klog.Infof("routeHost = " + routeHost)
		featuredAppScriptURL = "https://" + routeHost + "/kappnav-ui/openshift/featuredApp.js"
		appLauncherScriptURL = "https://" + routeHost + "/kappnav-ui/openshift/appLauncher.js"
		iconScriptURL = "https://" + routeHost + "/kappnav-ui/openshift/appNavIcon.css"
		klog.Info("kappNavFeaturedAppScript = " + featuredAppScriptURL)
		klog.Info("kappNavAppLauncherScript = " + appLauncherScriptURL)
		klog.Info("kappNavIconScript = " + iconScriptURL)
	}
}

// addString adds a string to an array if the array
// does not already contain the string
func addString(input []interface{}, str string) []interface{} {
	if containsString(input, str) {
		klog.Infof("addString output (unchanged) = %v", input)
		return input
	}
	output := append(input, str)
	if klog.V(3) {
		klog.Infof("addString output = %v", output)
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
		klog.Infof("removeString output (unchanged) = %v", input)
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
	if klog.V(3) {
		klog.Infof("removeString output = %v", output)
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
