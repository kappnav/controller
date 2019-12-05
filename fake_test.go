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
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	restClient "k8s.io/client-go/rest"
	"k8s.io/klog"
)

func readFile(fileName string) ([]byte, error) {
	ret := make([]byte, 0)
	file, err := os.Open(fileName)
	if err != nil {
		return ret, err
	}
	defer file.Close()
	input := bufio.NewScanner(file)
	for input.Scan() {
		for _, b := range input.Bytes() {
			ret = append(ret, b)
		}
	}
	return ret, nil
}

func readJSON(fileName string) (*unstructured.Unstructured, error) {
	bytes, err := readFile(fileName)
	if err != nil {
		return nil, err
	}
	var unstructuredObj = &unstructured.Unstructured{}
	err = unstructuredObj.UnmarshalJSON(bytes)
	if err != nil {
		return nil, err
	}
	return unstructuredObj, nil
}

func readOneResourceID(fileName string) (resourceID, error) {
	unstructuredObj, err := readJSON(fileName)
	if err != nil {
		return resourceID{}, fmt.Errorf("error in file %s, error: %s", fileName, err)
	}
	var resInfo = &resourceInfo{}
	parseResource(unstructuredObj, resInfo)
	var resource = resourceID{}
	resource.fileName = fileName
	resource.kind = resInfo.kind
	resource.gvr = resInfo.gvr
	resource.namespace = resInfo.namespace
	resource.name = resInfo.name
	resource.expectedStatus = Normal
	resource.flyover = "flyover"
	resource.flyoverNLS = "[ \"status.nls.key\", \"2\", \"2\", \"0\" ]"
	resource.resInfo = resInfo
	return resource, nil
}

/* read all resourceIDs and return a map of file name to resourceID */
func readResourceIDs(files []string) ([]resourceID, error) {
	var ret = make([]resourceID, 0)
	for _, file := range files {
		res, err := readOneResourceID(file)
		if err != nil {
			return nil, err
		}
		ret = append(ret, res)
	}
	return ret, nil
}

func kindToPlural(kind string) string {
	lowerKind := strings.ToLower(kind)
	if index := strings.LastIndex(lowerKind, "ss"); index == len(lowerKind)-2 {
		return lowerKind + "es"
	}
	if index := strings.LastIndex(lowerKind, "cy"); index == len(lowerKind)-2 {
		return lowerKind[0:index] + "cies"
	}
	return lowerKind + "s"
}

func testPluralToKind(t *testing.T) {
	var input = []string{"Ingress", "NetworkPolicy", "Deployment"}
	var output = []string{"ingresses", "networkpolicies", "deployments"}

	for index, inKind := range input {
		plural := kindToPlural(inKind)
		if strings.Compare(output[index], plural) != 0 {
			t.Errorf("kinToPlural: input: %s, expected output: %s, got: %s\n", inKind, output[index], plural)
		}
	}
}

// Populate resources into fake dynamic client, and populate fake discovery
func populateResources(toCreateResources []resourceID, dynInterf dynamic.Interface, fakeDisc *fakeDiscovery) error {
	if klog.V(3) {
		klog.Info("populateResources entry")
	}

	for _, toCreate := range toCreateResources {
		if klog.V(3) {
			klog.Infof("populateResources reading %s", toCreate.fileName)
		}
		obj, err := readJSON(toCreate.fileName)
		if err != nil {
			return err
		}
		gvk := obj.GroupVersionKind()
		resource := kindToPlural(gvk.Kind)

		// Add data for discovery
		if err = fakeDisc.addKind(gvk.Kind, gvk.Group, gvk.Version, gvk.Kind, resource); err != nil {
			if klog.V(3) {
				klog.Infof("populateResources unable to add kind %s", gvk.Kind)
			}
			return err
		}

		namespace := obj.GetNamespace()
		var nsinterf dynamic.NamespaceableResourceInterface
		nsinterf = dynInterf.Resource(schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: resource})
		var interf dynamic.ResourceInterface = nsinterf
		if strings.Compare(namespace, "") != 0 {
			interf = nsinterf.Namespace(namespace)
		}
		_, err = interf.Create(obj, metav1.CreateOptions{})

		if err != nil {
			if klog.V(3) {
				klog.Infof("populateResources exit err %s", err)
			}
			return err
		}
		if klog.V(3) {
			klog.Infof("populateResources success reading %s", toCreate.fileName)
		}
	}
	if klog.V(3) {
		klog.Info("populateResources exit success")
	}
	return nil
}

/****** BEGIN  fake discovery client */
type fakeAPIGroup struct {
	apiGroup     metav1.APIGroup
	apiResources map[string]*metav1.APIResource // map from kind to APIResource
}

type fakeDiscovery struct {
	// map of group name to APIGroup
	apiGroups map[string]*fakeAPIGroup
}

func newFakeDiscovery() *fakeDiscovery {
	var fd = &fakeDiscovery{
		apiGroups: make(map[string]*fakeAPIGroup),
	}
	return fd
}

type testDiscoveryData struct {
	kind    string
	group   string
	version string
	name    string
	plural  string
}

func TestFakeDiscovery(t *testing.T) {

	var testData = []testDiscoveryData{
		{kind: "AA1", group: "", version: "v1", name: "AA1", plural: "AA1s"},
		{kind: "AA2", group: "", version: "v1", name: "AA2", plural: "AA2s"},
		{kind: "BB1", group: "B", version: "v1beta1", name: "BB1", plural: "BB1s"},
		{kind: "BB2", group: "B", version: "v1beta1", name: "BB2", plural: "BB2s"},
		{kind: "BB3", group: "B", version: "v1beta1", name: "BB3", plural: "BB3s"},
	}
	fakeDisc := newFakeDiscovery()
	for _, data := range testData {
		if err := fakeDisc.addKind(data.kind, data.group, data.version, data.name, data.plural); err != nil {
			t.Fatal(err)
		}
	}

	var disc discovery.DiscoveryInterface = fakeDisc
	serverGroups, err := disc.ServerGroups()
	if err != nil {
		t.Fatal(err)
	}
	if sglen := len(serverGroups.Groups); sglen != 2 {
		t.Fatalf("server group length %d not 2\n", sglen)
	}

	gv1 := serverGroups.Groups[0].PreferredVersion.GroupVersion
	gv2 := serverGroups.Groups[1].PreferredVersion.GroupVersion
	var hasBuiltin bool
	var hasB bool
	if strings.Compare(gv1, "v1") == 0 || strings.Compare(gv2, "v1") == 0 {
		hasBuiltin = true
	}
	if strings.Compare(gv1, "B/v1beta1") == 0 || strings.Compare(gv2, "B/v1beta1") == 0 {
		hasB = true
	}
	if !hasBuiltin {
		t.Fatalf("Can't locate built-in API group\n")
	}
	if !hasB {
		t.Fatalf("Can't locate API group B\n")
	}

	resourceList1, err := disc.ServerResourcesForGroupVersion("v1")
	if err != nil {
		t.Fatal(err)
	}
	var apiResources1 = resourceList1.APIResources
	if len1 := len(apiResources1); len1 != 2 {
		t.Fatalf("Expecting 2 built-in resources, but got %d\n", len1)
	}

	resourceList2, err := disc.ServerResourcesForGroupVersion("B/v1beta1")
	if err != nil {
		t.Fatal(err)
	}
	var apiResources2 = resourceList2.APIResources
	if len2 := len(apiResources2); len2 != 3 {
		t.Fatalf("Expecting 3 reources for APi group B, but got %d\n", len2)
	}
}

/*
 Add a new API group. This implementation only adds the first one
*/
func (fd *fakeDiscovery) addAPIGroup(group string, version string) (*fakeAPIGroup, error) {
	existing, ok := fd.apiGroups[group]
	if !ok {
		// create new
		existing = &fakeAPIGroup{}
		fd.apiGroups[group] = existing
		existing.apiResources = make(map[string]*metav1.APIResource)
		existing.apiGroup = metav1.APIGroup{}
		existing.apiGroup.Kind = "APIVersion"
		existing.apiGroup.APIVersion = "v1"
		existing.apiGroup.Name = group
		var groupVersion string
		if strings.Compare(group, "") == 0 {
			groupVersion = version
		} else {
			groupVersion = group + "/" + version
		}
		existing.apiGroup.PreferredVersion = metav1.GroupVersionForDiscovery{
			GroupVersion: groupVersion,
			Version:      version}
		existing.apiGroup.Versions = make([]metav1.GroupVersionForDiscovery, 0)
		existing.apiGroup.Versions = append(existing.apiGroup.Versions, existing.apiGroup.PreferredVersion)
		existing.apiGroup.ServerAddressByClientCIDRs = make([]metav1.ServerAddressByClientCIDR, 0)
	} else {
		existingVersion := existing.apiGroup.PreferredVersion.Version
		if strings.Compare(existingVersion, version) != 0 {
			return nil, fmt.Errorf("Fake discovery client can't handle different versions of the same group.  group: %s, existing version: %s, new version: %s", group, existingVersion, version)
		}
	}
	return existing, nil
}

var noNamespace = map[string]bool{
	"ComponentStatus":                true,
	"Namespace":                      true,
	"Node":                           true,
	"PersistentVolume":               true,
	"MutatingWebhookConfiguration":   true,
	"ValidatingWebhookConfiguration": true,
	"CustomResourceDefinition":       true,
	"APIService":                     true,
	"TokenReview":                    true,
	"SelfSubjectAccessReview":        true,
	"SelfSubjectRulesReview":         true,
	"SubjectAccessReview":            true,
	"CertificateSigningRequest":      true,
	"PodSecurityPolicy":              true,
	"RuntimeClass":                   true,
	"ClusterRoleBinding":             true,
	"ClusterRole":                    true,
	"PriorityClass":                  true,
	"CSIDriver":                      true,
	"CSINode":                        true,
	"StorageClass":                   true,
	"VolumeAttachment":               true,
}

func isNamespaced(kind string) bool {
	if _, ok := noNamespace[kind]; ok {
		return false
	}
	return true
}

func (fd *fakeDiscovery) addKind(kind, group, version, name, plural string) error {
	if klog.V(3) {
		klog.Infof("addKind kind: %s, group: %s, name: %s, version: %s, plural:%s", kind, group, version, name, plural)
	}
	fakeGroup, err := fd.addAPIGroup(group, version)
	if err != nil {
		if klog.V(3) {
			klog.Infof("addKind failed %s", err)
		}
		return err
	}
	apiResource, ok := fakeGroup.apiResources[kind]
	if !ok {
		// kind not yet registered
		apiResource = &metav1.APIResource{}
		fakeGroup.apiResources[kind] = apiResource
		apiResource.Name = plural
		apiResource.SingularName = name
		apiResource.Namespaced = isNamespaced(kind)
		apiResource.Group = group
		apiResource.Version = version
		apiResource.Kind = kind
		apiResource.Verbs = make([]string, 0)
		apiResource.ShortNames = make([]string, 0)
		apiResource.Categories = make([]string, 0)
		// apiResource.StorageVersionHash = ""
	}
	return nil
}

func (fd *fakeDiscovery) RESTClient() restClient.Interface {
	return nil
}

func (fd *fakeDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	var apiGroupList = &metav1.APIGroupList{}
	apiGroupList.Kind = "APIGroupList"
	apiGroupList.APIVersion = "v1"
	apiGroupList.Groups = make([]metav1.APIGroup, 0)

	for _, fakeAPIGroup := range fd.apiGroups {
		apiGroupList.Groups = append(apiGroupList.Groups, fakeAPIGroup.apiGroup)
	}
	return apiGroupList, nil
}

func splitGroupVersion(groupVersion string) (group string, version string) {
	if strings.Contains(groupVersion, "/") {
		split := strings.Split(groupVersion, "/")
		group = split[0]
		version = split[1]
	} else {
		group = ""
		version = groupVersion
	}
	return
}

func (fd *fakeDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	group, _ := splitGroupVersion(groupVersion)
	var ret = &metav1.APIResourceList{}
	ret.Kind = "APIResourceList"
	ret.APIVersion = "v1"
	ret.APIResources = make([]metav1.APIResource, 0)
	fakeAPIGroup := fd.apiGroups[group]
	for _, apiResource := range fakeAPIGroup.apiResources {
		ret.APIResources = append(ret.APIResources, *apiResource)
	}
	return ret, nil
}

func (fd *fakeDiscovery) ServerResources() ([]*metav1.APIResourceList, error) {
	return nil, fmt.Errorf("Unsupported ServerResources")
}

func (fd *fakeDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, fmt.Errorf("Unsupported ServerGroupsAndResources")
}

func (fd *fakeDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, fmt.Errorf("Unsupported ServerPreferredResources")
}

func (fd *fakeDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, fmt.Errorf("Unsupported ServerPreferredNamespacedResources")
}

func (fd *fakeDiscovery) ServerVersion() (*version.Info, error) {
	return nil, fmt.Errorf("Unsupported ServerVersion")
}

func (fd *fakeDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, fmt.Errorf("Unsupported OpenAPISchema")
}

/****** END  fake discovery client */
