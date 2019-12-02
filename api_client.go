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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/klog"
)

// function to calculate component status.
type calculateComponentStatusFunc func(destUrl string, resInfo *resourceInfo) (status string, flyover string, flyOverNLS string, retErr error)

// http client to API srever
var client = &http.Client{}

/* Call API server to calculate component status */
func calculateComponentStatus(destURL string, resInfo *resourceInfo) (status string, flyover string, flyoverNLS string, retErr error) {

	status = ""
	flyover = ""
	flyoverNLS = ""
	query := url.QueryEscape(resInfo.namespace)
	apiVersion := resInfo.apiVersion
	if !strings.Contains(apiVersion, "/") {
		apiVersion = "/" + apiVersion
		if klog.V(4) {
			klog.Infof("calculateComponentStatus using apiVersion: %s instead of %s", apiVersion, resInfo.apiVersion)
		}
	}
	urlPath := destURL + "/kappnav/status/" + resInfo.name + "/" + resInfo.kind + "?namespace=" + query + "&apiversion=" + url.QueryEscape(apiVersion)
	if klog.V(4) {
		klog.Infof("calculateComponentStatus urlPath: %s", urlPath)
	}
	resp, retErr := client.Get(urlPath)
	if retErr != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("calculateComponentStatus failed. status: %s urlPath: %s", resp.Status, urlPath)
	}
	var result interface{}
	retErr = json.NewDecoder(resp.Body).Decode(&result)
	if retErr != nil {
		return
	}

	if klog.V(4) {
		klog.Infof("calculateComponentStatus: type is: %T\n", result)
	}
	switch result.(type) {
	case map[string]interface{}:
		resultMap := result.(map[string]interface{})
		var tmp interface{}
		var ok bool
		tmp, ok = resultMap["value"]
		if ok {
			status = tmp.(string)
		}
		tmp, ok = resultMap["flyover"]
		if ok {
			flyover = tmp.(string)
		}
		tmp, ok = resultMap["flyover.nls"]
		if ok {
			bytes, err := json.Marshal(tmp)
			if err != nil {
				klog.Errorf("Unable to marshal flyover.nls: %s", tmp)
			} else {
				flyoverNLS = string(bytes)
			}
		}
	default:
		retErr = fmt.Errorf("calculateComponentStatus failed: don't know how to process returned object of type %T", result)
	}

	if klog.V(4) {
		klog.Infof("calculateComponentStatus url: %s, kind: %s, namespace: %s, name: %s: status: %s, flyover: %s, flyovernLS: %s, err: %s", destURL, resInfo.kind, resInfo.namespace, resInfo.name, status, flyover, flyoverNLS, retErr)
	}
	return
}
