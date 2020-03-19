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
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

const (
	NoStatus = "NoStatus" // special indicator to check no status changed for resource
	redAlert = "Red Alert"
	problem  = "Problem"
	warning  = "Warning"
	Normal   = "Normal"
	unknown  = "Unknown"

	StatusFailureRate = 0.0
	BatchDuration     = time.Second
)

// JSON input files used for testing
const (
	CrdApplication           = "test_data/CRD_application.json"
	crdFoo                   = "test_data/CRD_foo.json"
	KappnavConfigFile        = "test_data/kappnav-config.json"
	appBookinfo              = "test_data/bookinfo.json"
	appDetails               = "test_data/details-app.json"
	appLoop2A                = "test_data/loop2A-app.json"
	appLoop2B                = "test_data/loop2B-app.json"
	appLoop4A                = "test_data/loop4A-app.json"
	appLoop4B                = "test_data/loop4B-app.json"
	appLoop4C                = "test_data/loop4C-app.json"
	appLoop4D                = "test_data/loop4D-app.json"
	appFoo                   = "test_data/foo-app.json"
	fooExample               = "test_data/example-foo.json"
	deploymentDetailsV1      = "test_data/details-v1.json"
	serviceDetails           = "test_data/details.json"
	ingressBookinfo          = "test_data/gateway.json"
	appProductpage           = "test_data/productpage-app.json"
	networkpolicyProductpage = "test_data/productpage-egress.json"
	deploymentProcuctpageV1  = "test_data/productpage-v1.json"
	serviceProductpage       = "test_data/productpage.json"
	appRatings               = "test_data/ratings-app.json"
	deploymentRatingsV1      = "test_data/ratings-v1.json"
	serviceRatings           = "test_data/ratings.json"
	appReviews               = "test_data/reviews-app.json"
	networkpolicyReviews     = "test_data/reviews-egress.json"
	deploymentReviewsV1      = "test_data/reviews-v1.json"
	deploymentReviewsV2      = "test_data/reviews-v2.json"
	deploymentReviewsV3      = "test_data/reviews-v3.json"
	serviceReview            = "test_data/reviews.json"
	aApp                     = "test_data/A-app.json"
	bApp                     = "test_data/B-app.json"
	abApp                    = "test_data/AB-app.json"
	abApp2                   = "test_data/AB-app2.json"
	abApp3                   = "test_data/AB-app3.json"
	cApp                     = "test_data/C-app.json"
	abDeployment             = "test_data/AB-deployment.json"
	abcDeployment            = "test_data/ABC-deployment.json"
	abcDeployment2           = "test_data/ABC-deployment2.json"
	cDeployment              = "test_data/C-deployment.json"
	aService                 = "test_data/A-service.json"
	bService                 = "test_data/B-service.json"
	nsListkAppNavConfigMap   = "test_data/ns_list_kappnav_config_map.json"
	nsNolistkAppNavConfigMap = "test_data/ns_nolist_kappnav_config_map.json"
	ns1AnnoApplication       = "test_data/ns1_anno_application.json"
	ns1App                   = "test_data/NS1-App.json"
	ns1aApp                  = "test_data/NS1A-App.json"
	ns1Service               = "test_data/NS1-Service.json"
	ns1Deployment            = "test_data/NS1-Deployment.json"
	ns2App                   = "test_data/NS2-App.json"
	ns2Service               = "test_data/NS2-Service.json"
	ns2Deployment            = "test_data/NS2-Deployment.json"
	ns3Application           = "test_data/ns3_application.json"
	ns3Service               = "test_data/ns3_service.json"
	ns3Deployment            = "test_data/ns3_deployment.json"
	ns4NoAnnoApplication     = "test_data/ns4_noanno_application.json"
	ns4Service               = "test_data/ns4_service.json"
	ns4Deployment            = "test_data/ns4_deployment.json"
	nonsApp                  = "test_data/NoNS-App.json"  // app that includes resource w/o namespace
	nonsNode                 = "test_data/NoNS-Node.json" // node resource, which has no namespace
	kappnavCRFile            = "test_data/kappnav-CR.json"
)

/**
*
*   Mocks for apiVersion and kind to GVR mapping. In an actual cluster these
*   are created dynamically via discovery
*
**/

var coreNetworkPolicyGVR = schema.GroupVersionResource{
	Group:    "extensions",
	Version:  "v1beta1",
	Resource: "networkpolicies",
}
var coreFooGVR = schema.GroupVersionResource{
	Group:    "samplecontroller.k8s.io",
	Version:  "v1alpha1",
	Resource: "foos",
}

var loglevel string

func init() {
	logger = NewLogger(false)  //create a new logger and set enableJSONLog false (log in plain text) 
	
	// set to none to avoid build exceeding log size
	logger.SetLogLevel(LogLevelNone)

	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, "Initializing kind to GVR map mocks")
	}

	// new flag to set the unit test log level
	flag.StringVar(&loglevel, "loglevel", "", "The log level setting for unit test")

	coreKindToGVR["NetworkPolicy"] = coreNetworkPolicyGVR
	coreKindToGVR["Foo"] = coreFooGVR
	coreKindToGVR["kappnav"] = coreKappNavGVR
}

func initControllerMaps(resController *ClusterWatcher) {
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, "initControllerMaps")
	}

	resController.apiVersionKindToGVR.Store("v1/Service", coreServiceGVR)
	resController.apiVersionKindToGVR.Store("apps/v1/Deployment", coreDeploymentGVR)
	resController.apiVersionKindToGVR.Store("route.openshift.io/v1/Route", coreRouteGVR)
	resController.apiVersionKindToGVR.Store("v1/ConfigMap", coreConfigMapGVR)
	resController.apiVersionKindToGVR.Store("v1/Secret", coreSecretGVR)
	resController.apiVersionKindToGVR.Store("core/v1/Volume", coreVolumeGVR)
	resController.apiVersionKindToGVR.Store("v1/PersistentVolumeClaim", corePersistentVolumeClaimGVR)
	resController.apiVersionKindToGVR.Store("apiextensions.k8s.io/v1beta1/CustomResourceDefinition", coreCustomResourceDefinitionGVR)
	resController.apiVersionKindToGVR.Store("app.k8s.io/v1beta1/Application", coreApplicationGVR)
	resController.apiVersionKindToGVR.Store("apps/v1/StatefulSet", coreStatefulSetGVR)
	resController.apiVersionKindToGVR.Store("extensions/v1beta1/Ingress", coreIngressGVR)
	resController.apiVersionKindToGVR.Store("extensions/v1beta1/NetworkPolicy", coreNetworkPolicyGVR)
	resController.apiVersionKindToGVR.Store("samplecontroller.k8s.io/v1alpha1/Foo", coreFooGVR)
	resController.apiVersionKindToGVR.Store("batch/v1/Job", coreJobGVR)
	resController.apiVersionKindToGVR.Store("v1/ServiceAccount", coreServiceAccountGVR)
	resController.apiVersionKindToGVR.Store("rbac.authorization.k8s.io/v1/ClusterRole", coreClusterRoleGVR)
	resController.apiVersionKindToGVR.Store("rbac.authorization.k8s.io/v1/ClusterRoleBinding", coreClusterRoleBindingGVR)
	resController.apiVersionKindToGVR.Store("rbac.authorization.k8s.io/v1/Role", coreRoleGVR)
	resController.apiVersionKindToGVR.Store("rbac.authorization.k8s.io/v1/RoleBinding", coreRoleBindingGVR)
	resController.apiVersionKindToGVR.Store("storage.k8s.io/v1/StorageClass", coreStorageClassGVR)
	resController.apiVersionKindToGVR.Store("v1/Endpoint", coreEndpointGVR)
	resController.apiVersionKindToGVR.Store("v1/Node", coreNodeGVR)
	resController.apiVersionKindToGVR.Store("v1/Kappnav", coreKappNavGVR)
}

func beforeTest() {
	setTestLogLevel(loglevel)
}

func setTestLogLevel(loglevel string) {
	loglevels := []string{"none", "warning", "error", "info", "debug", "entry", "all"}
	if len(loglevel) > 0 {
		str := strings.ToLower(loglevel)
		_, found := Find(loglevels, str)
		if found {
			//set logging level
			fmt.Println("The unit test log level is set to:", loglevel)
			SetLoggingLevel(str)
		} else {
			fmt.Println("The specified loglevel value is not valid. Valid values are: none, warning, error, info, debug, entry, all")
		}
	}
}

// returns function to get component status, scoped by the testActions
func newComponentStatusFunc(ta *testActions, failRate float32) calculateComponentStatusFunc {
	var testActs = ta
	var failureRate = failRate
	return func(destUrl string, resInfo *resourceInfo) (status string, flyover string, flyoverNLS string, retErr error) {
		if rand.Float32() < failureRate {
			return "", "", "", fmt.Errorf("In test %s, random error introduced when getting status for %s %s %s", testActs.testName, resInfo.kind, resInfo.namespace, resInfo.name)
		}
		return testActs.getStatus(resInfo.kind, resInfo.namespace, resInfo.name)
	}
}

// Create a new ClusterWatcher for unit test environment
// pre-populating it with resources
func createClusterWatcher(resources []resourceID, testActions *testActions, failureRate float32) (*ClusterWatcher, error) {
	if logger.IsEnabled(LogTypeEntry) {
		logger.Log(CallerName(), LogTypeEntry, "")
	}

	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	fakeDiscovery := newFakeDiscovery()
	err := populateResources(resources, dynClient, fakeDiscovery)
	if err != nil {
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("Exiting, error: %v", err))
		}
		return nil, err
	}

	plugin := &ControllerPlugin{
		dynClient, fakeDiscovery, BatchDuration, newComponentStatusFunc(testActions, failureRate)}
	resController, err := NewClusterWatcher(plugin)
	if err != nil {
		if logger.IsEnabled(LogTypeExit) {
			logger.Log(CallerName(), LogTypeExit, fmt.Sprintf("Error calling NewClusterWatcher: %s", err))
		}
		return resController, err
	}
	initControllerMaps(resController)

	testActions.setClusterWatcher(resController)
	if logger.IsEnabled(LogTypeExit) {
		logger.Log(CallerName(), LogTypeExit, "success")
	}

	return resController, nil
}

// Test we can populate the cache with a bunch of resources
func TestPopulateResources(t *testing.T) {
	testName := "TestPopulateResources"

	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{}

	var files = []string{
		KappnavConfigFile,
		CrdApplication,
		appBookinfo,
		appDetails,
		deploymentDetailsV1,
		serviceDetails,
		ingressBookinfo,
		appProductpage,
		networkpolicyProductpage,
		deploymentProcuctpageV1,
		serviceProductpage,
		appRatings,
		deploymentRatingsV1,
		serviceRatings,
		appReviews,
		networkpolicyReviews,
		deploymentReviewsV1,
		deploymentReviewsV2,
		deploymentReviewsV3,
		serviceReview,
		crdFoo,
		fooExample,
		appFoo,
		kappnavCRFile,
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	/* iteration 1: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	/* create a watcher that populates all resources */
	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// ensure we can find each resource
	for _, res := range iteration0IDs {
		exists, _ := resourceExists(clusterWatcher, res)
		if !exists {
			t.Fatal(fmt.Errorf("can't find resource for %s\n,", res.fileName))
		}
	}

	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

/* helper to test application.
   positiveTest: true if actions should succeed. False if it should fail
*/
func oneAppHelper(t *testing.T, testName string, positiveTest bool) error {

	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:   true,
		"Service":     true,
		"Deployment":  true,
		"StatefulSet": true,
	}

	// resources to pre-populate
	var files = []string{
		CrdApplication,
		appProductpage,
		networkpolicyProductpage,
		deploymentProcuctpageV1,
		serviceProductpage,
		KappnavConfigFile,
		kappnavCRFile,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: set deploymet to Warning
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[1].expectedStatus = warning // application now warning
	iteration1IDs[3].expectedStatus = warning
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: set service to Problem
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[1].expectedStatus = problem // application now problem
	iteration2IDs[4].expectedStatus = problem
	testActions.addIteration(iteration2IDs, emptyIDs)

	// iteration 3: set service to Warning
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	iteration3IDs[1].expectedStatus = warning // application now warning
	iteration3IDs[4].expectedStatus = warning
	testActions.addIteration(iteration3IDs, emptyIDs)

	// iteration 4: set everything back to normal
	testActions.addIteration(iteration0IDs, emptyIDs)

	/* iteration 5: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	var failureRate float32
	if positiveTest {
		failureRate = StatusFailureRate
	} else {
		failureRate = 1.0
	}
	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, failureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if positiveTest {
		if err != nil {
			return err
		}
		return nil
	}
	if err == nil {
		return fmt.Errorf("negative test, but actions passed")
	}
	return nil
}

func TestOneApp(t *testing.T) {
	testName := "TestOneApp"
	beforeTest()
	if err := oneAppHelper(t, testName, true); err != nil {
		t.Fatal(err)
	}
}

/* Test nested application */
func TestNestedApp(t *testing.T) {
	testName := "TestNestedApp"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:     true,
		"Ingress":       true,
		"Service":       true,
		"Deployment":    true,
		"StatefulSet":   true,
		"NetworkPolicy": true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ CrdApplication,
		/* 1 */ appBookinfo,
		/* 2 */ appProductpage,
		/* 3 */ appDetails,
		/* 4 */ appRatings,
		/* 5 */ appReviews,
		/* 6 */ deploymentDetailsV1,
		/* 7 */ deploymentProcuctpageV1,
		/* 8 */ deploymentRatingsV1,
		/* 9 */ deploymentReviewsV1,
		/* 10 */ deploymentReviewsV2,
		/* 11 */ deploymentReviewsV3,
		/* 12 */ ingressBookinfo,
		/* 13 */ networkpolicyProductpage,
		/* 14 */ networkpolicyReviews,
		/* 15 */ serviceDetails,
		/* 16 */ serviceProductpage,
		/* 17 */ serviceRatings,
		/* 18 */ serviceReview,
		KappnavConfigFile,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: set details app to redAlert
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[1].expectedStatus = redAlert // bookinfo now redAlert
	iteration1IDs[3].expectedStatus = redAlert // details app now redAlert
	iteration1IDs[15].expectedStatus = redAlert
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: set details to Warning but changing its deployment
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[1].expectedStatus = warning // bookinfo now problem
	iteration2IDs[3].expectedStatus = warning // details app now warning
	iteration2IDs[15].expectedStatus = Normal // details service NRMAL
	iteration2IDs[6].expectedStatus = warning // but details deployment warning
	testActions.addIteration(iteration2IDs, emptyIDs)

	// iteration 3: set everything back to normal
	testActions.addIteration(iteration0IDs, emptyIDs)

	/* iteration 4: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

/* Test adding resources */
func TestAddResource(t *testing.T) {
	testName := "TestAddResource"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:     true,
		"Ingress":       true,
		"Service":       true,
		"Deployment":    true,
		"StatefulSet":   true,
		"NetworkPolicy": true,
		//		"Kappnav":       true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ KappnavConfigFile,
		/* 1 */ CrdApplication,
		/* 2 */ appBookinfo,
		/* 3 */ appProductpage,
		/* 4 */ appDetails,
		/* 5 */ appReviews,
		/* 6 */ deploymentDetailsV1,
		/* 7 */ deploymentProcuctpageV1,
		/* 8 */ deploymentReviewsV1,
		/* 9 */ deploymentReviewsV2,
		/* 10 */ serviceDetails,
		/* 11 */ serviceProductpage,
		/* 12 */ serviceReview,
		/* 13 */ networkpolicyProductpage,
		/* 14 */ ingressBookinfo,
		/* 15 */ networkpolicyReviews,
		//		/* 16 */ kappnavCRFile,
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: Add deploymentReviewsV3
	// /* 16 */deploymentReviewsV3,
	res, err := readOneResourceID(deploymentReviewsV3)
	if err != nil {
		t.Fatal(err)
	}
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs = append(iteration1IDs, res)
	iteration1IDs[2].expectedStatus = warning  // bookfino warning
	iteration1IDs[5].expectedStatus = warning  // review app warning
	iteration1IDs[16].expectedStatus = warning // new deployment starts with warning status
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: stabilize the new deployment to Normal
	arrayLength = len(iteration1IDs)
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[2].expectedStatus = Normal  // bookfino Normal
	iteration2IDs[5].expectedStatus = Normal  // review app Normal
	iteration2IDs[16].expectedStatus = Normal // new deployment Normal
	testActions.addIteration(iteration2IDs, emptyIDs)

	/* iteration 3: add a new app */
	var newFiles = []string{
		/* 17 */ appRatings,
		/* 18 */ deploymentRatingsV1,
		/* 19 */ serviceRatings,
	}
	newResources, err := readResourceIDs(newFiles)
	if err != nil {
		t.Fatal(err)
	}
	arrayLength = len(iteration2IDs)
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	for _, newRes := range newResources {
		iteration3IDs = append(iteration3IDs, newRes)
	}
	iteration3IDs[2].expectedStatus = warning  // bookfino now warning due to ratings app
	iteration3IDs[17].expectedStatus = warning // ratings app warning
	iteration3IDs[18].expectedStatus = warning // ratings deployment warning
	testActions.addIteration(iteration3IDs, emptyIDs)

	/* iteration 4: everything back to normal */
	arrayLength = len(iteration3IDs)
	var iteration4IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration4IDs, iteration3IDs)
	iteration4IDs[2].expectedStatus = Normal  // bookfino app Normal
	iteration4IDs[17].expectedStatus = Normal // ratings app Normal
	iteration4IDs[18].expectedStatus = Normal // ratings deployment Normal
	testActions.addIteration(iteration4IDs, emptyIDs)

	/* iteration 7: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

/* Test delete resource */
func TestDeleteResource(t *testing.T) {
	testName := "TestDeleteResource"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:     true,
		"Ingress":       true,
		"Service":       true,
		"Deployment":    true,
		"StatefulSet":   true,
		"NetworkPolicy": true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ KappnavConfigFile,
		/* 1 */ CrdApplication,
		/* 4 */ appBookinfo,
		/* 5 */ appProductpage,
		/* 6 */ appDetails,
		/* 7 */ appRatings,
		/* 8 */ deploymentDetailsV1,
		/* 9 */ deploymentProcuctpageV1,
		/* 10 */ deploymentRatingsV1,
		/* 11 */ ingressBookinfo,
		/* 12 */ networkpolicyProductpage,
		/* 13 */ networkpolicyReviews,
		/* 14 */ serviceDetails,
		/* 15 */ serviceProductpage,
		/* 16 */ serviceRatings,
		/* 17 */ serviceReview,
		/* 18 */ deploymentReviewsV1,
		/* 19 */ appReviews,
		/* 20 */ deploymentReviewsV2,
		/* 21 */ deploymentReviewsV3,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	testActions := newTestActions(testName, kindsToCheckStatus)

	/* Iteration 0 */
	iteration0IDs[2].expectedStatus = problem  // bookinfo problem due to review app
	iteration0IDs[17].expectedStatus = problem // review app problem due to deploymentReviewsV3
	iteration0IDs[18].expectedStatus = warning // deploymentReviewsV2 is WARING
	iteration0IDs[19].expectedStatus = problem // deploymentReviewsV3 is problem
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	/* iteration 1: delete deploymentReviewsV3 */
	arrayLength := len(iteration0IDs) - 1
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[2].expectedStatus = warning  // bookfino now warning
	iteration1IDs[17].expectedStatus = warning // review app now warning deu to deploymentReviewsV3 being deleted
	testActions.addIteration(iteration1IDs, emptyIDs)

	/* iteration 2: delete deploymentReviewsV2 */
	arrayLength = len(iteration1IDs) - 1
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[2].expectedStatus = Normal  // bookfino now Normal
	iteration2IDs[17].expectedStatus = Normal // reviews now Normal deu to deploymentReviewsV2 being deleted
	testActions.addIteration(iteration2IDs, emptyIDs)

	/* iteration 3: set deploymentReviewsV1 to warning */
	arrayLength = len(iteration2IDs)
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	iteration3IDs[2].expectedStatus = warning  // bookfino now Normal
	iteration3IDs[16].expectedStatus = warning // deploymentReviewsV1 now warning
	iteration3IDs[17].expectedStatus = warning // reviews now Normal deu to deploymentReviewsV1 being warning
	testActions.addIteration(iteration3IDs, emptyIDs)

	/* iteration 4:  delet review app */
	arrayLength = len(iteration3IDs) - 1
	var iteration4IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration4IDs, iteration3IDs)
	iteration4IDs[2].expectedStatus = Normal // bookfino now Normal due to review app being deleted
	testActions.addIteration(iteration4IDs, emptyIDs)

	/* iteration 5: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

/* Test receusive depeeency involving 2 applications
In this test, the application loop2A depends on details and loop2B
The application loop2B depends on ratings and loop2A
*/
func TestLoop2(t *testing.T) {
	testName := "TestLoop2"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:     true,
		"Ingress":       true,
		"Service":       true,
		"Deployment":    true,
		"StatefulSet":   true,
		"NetworkPolicy": true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ CrdApplication,
		/* 1 */ appDetails,
		/* 2 */ appRatings,
		/* 3 */ deploymentDetailsV1,
		/* 4 */ deploymentRatingsV1,
		/* 5 */ serviceDetails,
		/* 6 */ serviceRatings,
		/* 7 */ appLoop2A,
		/* 8 */ appLoop2B,
		KappnavConfigFile,
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: set details app to warning
	// Both loop2A and loop2B apps also become warning
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[3].expectedStatus = warning // set deployment for details to warning
	iteration1IDs[1].expectedStatus = warning // details app now warning
	iteration1IDs[7].expectedStatus = warning // loop2A now warning
	iteration1IDs[8].expectedStatus = warning // loop2B now warning
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: set rating app to problem.
	// Both loop2A and loop2B apps also become  problem
	arrayLength = len(iteration1IDs)
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[4].expectedStatus = problem // set deployment for ratings to problem
	iteration2IDs[2].expectedStatus = problem // ratings app now problem
	iteration2IDs[7].expectedStatus = problem // loop2A now problem
	iteration2IDs[8].expectedStatus = problem // loop2B now problem
	testActions.addIteration(iteration2IDs, emptyIDs)

	// Iteration 3: set rating app to warning
	// Both loop2A and loop2B apps also become warning
	arrayLength = len(iteration2IDs)
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	iteration3IDs[4].expectedStatus = warning // set deployment for ratings to warning
	iteration3IDs[2].expectedStatus = warning // ratings app now warning
	iteration3IDs[7].expectedStatus = warning // loop2A now warning
	iteration3IDs[8].expectedStatus = warning // loop2B now warning
	testActions.addIteration(iteration3IDs, emptyIDs)

	// iteration 4: set details app to Normal
	// Both loop2A and loop2B remains warning
	arrayLength = len(iteration3IDs)
	var iteration4IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration4IDs, iteration3IDs)
	iteration4IDs[3].expectedStatus = Normal // set deployment for details to Normal
	iteration4IDs[1].expectedStatus = Normal // details app now Normal
	testActions.addIteration(iteration4IDs, emptyIDs)

	// Iteration 5: set rating app to Normal
	// Both loop2A and loop2B apps also become Normal
	arrayLength = len(iteration4IDs)
	var iteration5IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration5IDs, iteration4IDs)
	iteration5IDs[4].expectedStatus = Normal // set deployment for ratings to warning
	iteration5IDs[2].expectedStatus = Normal // ratings app now Normal
	iteration5IDs[7].expectedStatus = Normal // loop2A now Normal
	iteration5IDs[8].expectedStatus = Normal // loop2B now Normal
	testActions.addIteration(iteration5IDs, emptyIDs)

	/* iteration 6: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

/* Test receusive dependency involving 4 applications
In this test,
 - application loop2A depends on details and loop2B
 - application loop2B depends on ratings and loop2C
 - application loop2C depends on  productpage, loop2A, and loop2D
 - application loop2D depends on  reviews, loop2A, and loop2B
 The cycles are:
   loop2A > loop2B > loop3C > loop2D > loop2A
   loop2A > loop2B > loop2C > loop2A
   loop2B > loop2C > loop2D > loop2B

  loop2A <-- loop2D
     |  <    /    ^
     |    \ /     |
     |     x      |
     |    / \     |
     v  <    \    |
   loop2B ----> loop2C
*/
func TestLoop4(t *testing.T) {
	testName := "TestLoop4"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:     true,
		"Ingress":       true,
		"Service":       true,
		"Deployment":    true,
		"StatefulSet":   true,
		"NetworkPolicy": true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ CrdApplication,
		/* 3 */ appBookinfo,
		/* 4 */ appProductpage,
		/* 5 */ appDetails,
		/* 6 */ appRatings,
		/* 11 */ deploymentDetailsV1,
		/* 12 */ deploymentProcuctpageV1,
		/* 13 */ deploymentRatingsV1,
		/* 14 */ ingressBookinfo,
		/* 15 */ networkpolicyProductpage,
		/* 16 */ networkpolicyReviews,
		/* 17 */ serviceDetails,
		/* 18 */ serviceProductpage,
		/* 19 */ serviceRatings,
		/* 20 */ serviceReview,
		/* 21 */ deploymentReviewsV1,
		/* 22 */ appReviews,
		/* 23 */ deploymentReviewsV2,
		/* 24 */ deploymentReviewsV3,
		/* 25 */ appLoop4A,
		/* 26 */ appLoop4B,
		/* 27 */ appLoop4C,
		/* 28 */ appLoop4D,
		KappnavConfigFile,
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: set productpage app to warning
	// loop4A, loop4B, loop4C, loop4D should also be warning
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[6].expectedStatus = warning // set deployment for productpage to warning
	iteration1IDs[2].expectedStatus = warning // app productpage now warning
	iteration1IDs[1].expectedStatus = warning // app bookinfo now warning
	iteration1IDs[19].expectedStatus = warning
	iteration1IDs[20].expectedStatus = warning
	iteration1IDs[21].expectedStatus = warning
	iteration1IDs[22].expectedStatus = warning
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: set reviews app to warning
	arrayLength = len(iteration1IDs)
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[15].expectedStatus = warning // set deployment for reviews to warning
	iteration2IDs[16].expectedStatus = warning // app reviews now warning
	testActions.addIteration(iteration2IDs, emptyIDs)

	// iteration 3: set reviews app to problem
	arrayLength = len(iteration2IDs)
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	iteration3IDs[15].expectedStatus = problem // set deployment for reviews to problem
	iteration3IDs[16].expectedStatus = problem // app reviews now problem
	iteration3IDs[1].expectedStatus = problem  // app bookinfo now problem
	iteration3IDs[19].expectedStatus = problem
	iteration3IDs[20].expectedStatus = problem
	iteration3IDs[21].expectedStatus = problem
	iteration3IDs[22].expectedStatus = problem
	testActions.addIteration(iteration3IDs, emptyIDs)

	// iteration 4: set ratings app to warning
	arrayLength = len(iteration3IDs)
	var iteration4IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration4IDs, iteration3IDs)
	iteration4IDs[7].expectedStatus = warning // set deployment for ratingsapp to WARINNG
	iteration4IDs[4].expectedStatus = warning // app ratings now warning
	testActions.addIteration(iteration4IDs, emptyIDs)

	// iteration 5: set reviews app to warning
	arrayLength = len(iteration4IDs)
	var iteration5IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration5IDs, iteration4IDs)
	iteration5IDs[15].expectedStatus = warning // set deployment for reviews to WARINNG
	iteration5IDs[16].expectedStatus = warning // app reviews now warning
	iteration5IDs[1].expectedStatus = warning  // app bookinfo now warning
	iteration5IDs[19].expectedStatus = warning
	iteration5IDs[20].expectedStatus = warning
	iteration5IDs[21].expectedStatus = warning
	iteration5IDs[22].expectedStatus = warning
	testActions.addIteration(iteration5IDs, emptyIDs)

	// iteration 6: set productpage app to Normal
	arrayLength = len(iteration5IDs)
	var iteration6IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration6IDs, iteration5IDs)
	iteration6IDs[6].expectedStatus = Normal // set deployment for productpage to Normal
	iteration6IDs[2].expectedStatus = Normal // app productpage now Normal
	testActions.addIteration(iteration6IDs, emptyIDs)

	// iteration 7: set reviews app to Normal
	arrayLength = len(iteration6IDs)
	var iteration7IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration7IDs, iteration6IDs)
	iteration7IDs[15].expectedStatus = Normal // set deployment for reviews to Normal
	iteration7IDs[16].expectedStatus = Normal // app reviews now Normal
	testActions.addIteration(iteration7IDs, emptyIDs)

	// iteration 8: set ratings app to Normal
	arrayLength = len(iteration7IDs)
	var iteration8IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration8IDs, iteration7IDs)
	iteration8IDs[7].expectedStatus = Normal // set deployment for ratings to Normal
	iteration8IDs[4].expectedStatus = Normal // app ratings now Normal
	iteration8IDs[1].expectedStatus = Normal // app bookinfo now Normal
	iteration8IDs[19].expectedStatus = Normal
	iteration8IDs[20].expectedStatus = Normal
	iteration8IDs[21].expectedStatus = Normal
	iteration8IDs[22].expectedStatus = Normal
	testActions.addIteration(iteration8IDs, emptyIDs)

	/* iteration 9: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestChangeSpec(t *testing.T) {

	testName := "TestChangeSpec"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:     true,
		"Ingress":       true,
		"Service":       true,
		"Deployment":    true,
		"StatefulSet":   true,
		"NetworkPolicy": true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ CrdApplication,
		/* 2 */ aApp,
		/* 3 */ bApp,
		/* 4 */ abApp,
		/* 5 */ cApp,
		/* 6 */ abDeployment,
		/* 7 */ abcDeployment,
		/* 8 */ cDeployment,
		/* 9 */ aService,
		/* 10 */ bService,
		KappnavConfigFile,
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Start with:
	            A-app B-app      C-app
	             /\   / \          |\
	            /  \ /   \         | \
	    A-service AB-app B-service |  \
	              /  \             |   \
	             /    \            |    \
	   AB-deployment  ABC-deployment   C-deployment
	*/
	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: set ABC-deployment to problem
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[6].expectedStatus = problem // set deployment for productpage to warning
	// all apps now problem
	iteration1IDs[1].expectedStatus = problem
	iteration1IDs[2].expectedStatus = problem
	iteration1IDs[3].expectedStatus = problem
	iteration1IDs[4].expectedStatus = problem
	testActions.addIteration(iteration1IDs, emptyIDs)

	/* iteration 2: change the labels of ABC-deployment to match C app only
	            A-app B-app      C-app
	             /\   / \          |\
	            /  \ /   \         | \
	    A-service AB-app B-service |  \
	              /                |   \
	             /                 |    \
	   AB-deployment  ABC-deployment   C-deployment
	*/
	arrayLength = len(iteration1IDs)
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[6].fileName = abcDeployment2 // change the label
	// C-App should remain problem, others are normal
	iteration2IDs[1].expectedStatus = Normal
	iteration2IDs[2].expectedStatus = Normal
	iteration2IDs[3].expectedStatus = Normal
	testActions.addIteration(iteration2IDs, emptyIDs)

	/* iteration 3: change the labels of ABC-deployment back
	            A-app B-app      C-app
	             /\   / \          |\
	            /  \ /   \         | \
	    A-service AB-app B-service |  \
	              /  \             |   \
	             /    \            |    \
	   AB-deployment  ABC-deployment   C-deployment
	*/
	arrayLength = len(iteration2IDs)
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	iteration3IDs[7].fileName = abcDeployment // change the label
	// all apps now problem
	iteration3IDs[1].expectedStatus = problem
	iteration3IDs[2].expectedStatus = problem
	iteration3IDs[3].expectedStatus = problem
	iteration3IDs[4].expectedStatus = problem
	testActions.addIteration(iteration3IDs, emptyIDs)

	/* iteration 4: change the selectors of AB-app  to not select anything
	            A-app B-app      C-app
	             /\   / \          |\
	            /  \ /   \         | \
	    A-service AB-app B-service |  \
	                               |   \
	                               |    \
	   AB-deployment  ABC-deployment   C-deployment
	*/
	arrayLength = len(iteration3IDs)
	var iteration4IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration4IDs, iteration3IDs)
	// AB-app is UNKOWN, but the others remain problem
	iteration4IDs[3].fileName = abApp2
	iteration4IDs[1].expectedStatus = unknown
	iteration4IDs[2].expectedStatus = unknown
	iteration4IDs[3].expectedStatus = unknown
	testActions.addIteration(iteration4IDs, emptyIDs)

	// iteration 5: change AB-app  back
	arrayLength = len(iteration4IDs)
	var iteration5IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration5IDs, iteration4IDs)
	iteration5IDs[3].fileName = abApp
	// all apps now problem
	iteration5IDs[1].expectedStatus = problem
	iteration5IDs[2].expectedStatus = problem
	iteration5IDs[3].expectedStatus = problem
	testActions.addIteration(iteration5IDs, emptyIDs)

	// iteration 6: change AB-app label to be selected only by A-app
	arrayLength = len(iteration5IDs)
	var iteration6IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration6IDs, iteration5IDs)
	iteration6IDs[3].fileName = abApp3
	iteration6IDs[2].expectedStatus = Normal // B-app now normal
	testActions.addIteration(iteration6IDs, emptyIDs)

	/* iteration 7: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNegative(t *testing.T) {
	testName := "TestNegative"
	beforeTest()
	if err := oneAppHelper(t, testName, false); err != nil {
		t.Fatal(err)
	}
}

func TestCRDFoo(t *testing.T) {
	testName := "TestCRDFoo"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION: true,
		"Foo":       true,
	}

	// resources to pre-populate
	var files = []string{
		KappnavConfigFile,
		CrdApplication,
		appFoo,
		fooExample,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: set example-foo to warning
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[3].expectedStatus = warning
	iteration1IDs[2].expectedStatus = warning
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: remove Example-foo
	arrayLength = len(iteration1IDs) - 1
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[2].expectedStatus = unknown
	testActions.addIteration(iteration2IDs, emptyIDs)

	// iteration 3: remove Foo CRD
	arrayLength = len(iteration2IDs) - 1
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	testActions.addIteration(iteration3IDs, emptyIDs)

	// iteration 4: add it all back. Evertying should test normal
	arrayLength = len(iteration0IDs)
	var iteration4IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration4IDs, iteration0IDs)
	testActions.addIteration(iteration4IDs, emptyIDs)

	/* iteration 5: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNamespace(t *testing.T) {
	testName := "TestNamespace"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
		"Service":    true,
	}

	// starting resources to pre-populate
	var files = []string{
		/* 0 */ KappnavConfigFile,
		/* 1 */ CrdApplication,
		/* 2 */ ns1Service,
		/* 3 */ ns1Deployment,
		/* 4 */ ns1App,
		/* 5 */ ns2Service,
		/* 6 */ ns2Deployment,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: only namespace 1 resources should have status */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	iteration0IDs[5].expectedStatus = NoStatus // don't expect status to be checked for namespace 2
	iteration0IDs[6].expectedStatus = NoStatus // don't expect status to be checked for namespace 2
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: transition app in namespace 1 to Warning
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[2].expectedStatus = warning
	iteration1IDs[4].expectedStatus = warning
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: add app in namespace 2. NS1 app should still be Warning, while NS2 App should be Normal
	files1 := append(files, ns2App)
	iteration2IDs, err := readResourceIDs(files1)
	if err != nil {
		t.Fatal(err)
	}
	iteration2IDs[2].expectedStatus = warning
	iteration2IDs[4].expectedStatus = warning
	testActions.addIteration(iteration2IDs, emptyIDs)

	// iteration 3: Make NS1 app normal, while NS2 app Problem
	arrayLength = len(iteration2IDs)
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	iteration3IDs[2].expectedStatus = Normal
	iteration3IDs[4].expectedStatus = Normal
	iteration3IDs[6].expectedStatus = problem
	iteration3IDs[7].expectedStatus = problem
	testActions.addIteration(iteration3IDs, emptyIDs)

	// iteration 4: Make NS1 app Problem, while NS2 app Warning
	var iteration4IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration4IDs, iteration3IDs)
	iteration4IDs[3].expectedStatus = problem
	iteration4IDs[4].expectedStatus = problem
	iteration4IDs[5].expectedStatus = warning
	iteration4IDs[6].expectedStatus = Normal
	iteration4IDs[7].expectedStatus = warning
	testActions.addIteration(iteration4IDs, emptyIDs)

	/* iteration 4: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

/****************************************************************

            NAMESPACES TEST VARIATION MATRIX

                        kappnav-config app-namespaces:
                          n1, n2               ""
  A               -------------------------------------------
  p  namespace:   |  Status:           |  Status:           |
  p    n4         |                    |    n4              |
  l  annotation:  |  No status:        |  No status:        |
  i    ""         |    n1, n2, n3, n4  |    n1, n2, n3      |
  c               -------------------------------------------
  a  namespace:   |  Status:           |  Status:           |
  t    n1         |    n1, n2          |    n1, n2, n3      |
  i  annotation:  |  No status:        |  No status:        |
  o    n2, n3     |    n3, n4          |    n4              |
  n               -------------------------------------------

****************************************************************/

// TestNamespace1 tests:
// - kappnav ConfigMap with app-namespaces = ""
// - Application in ns4 with kappnav.component.namespaces = ""
// - Services/Deployments in ns1, ns2, ns3, ns4
// - Expected result: No status returned by any resources
func TestNamespace1(t *testing.T) {
	testName := "TestNamespace1"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		"Application": true,
		"Deployment":  true,
		"Service":     true,
	}

	// starting resources to pre-populate
	var files = []string{
		nsListkAppNavConfigMap,
		CrdApplication,
		ns4NoAnnoApplication,
		ns1Service,
		ns1Deployment,
		ns2Service,
		ns2Deployment,
		ns3Service,
		ns3Deployment,
		ns4Service,
		ns4Deployment,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: no resources should have status */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}

	iteration0IDs[2].expectedStatus = NoStatus // ns4 Application
	iteration0IDs[3].expectedStatus = NoStatus // ns1 Service
	iteration0IDs[4].expectedStatus = NoStatus
	iteration0IDs[5].expectedStatus = NoStatus // ns2 Service
	iteration0IDs[6].expectedStatus = NoStatus
	iteration0IDs[7].expectedStatus = NoStatus // ns3 Service
	iteration0IDs[8].expectedStatus = NoStatus
	iteration0IDs[9].expectedStatus = NoStatus // ns4 Service
	iteration0IDs[10].expectedStatus = NoStatus
	testActions.addIteration(iteration0IDs, emptyIDs)

	/* iteration 1: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// run all the test actions
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

// TestNamespace2 tests:
// - kappnav ConfigMap with app-namespaces = "ns1, ns2"
// - Application in ns1 with kappnav.component.namespaces = "ns2, ns3"
// - Services/Deployments in ns1, ns2, ns3, ns4
// - Expected result:
//     Status returned for ns1, ns2 resources
//     No status returned for ns3, ns4 resources
func TestNamespace2(t *testing.T) {
	testName := "TestNamespace2"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		"Application": true,
		"Deployment":  true,
		"Service":     true,
	}

	// starting resources to pre-populate
	var files = []string{
		nsListkAppNavConfigMap,
		CrdApplication,
		ns1AnnoApplication,
		ns1Service,
		ns1Deployment,
		ns2Service,
		ns2Deployment,
		ns3Service,
		ns3Deployment,
		ns4Service,
		ns4Deployment,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: only ns1 and ns2 resources should have status */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}

	iteration0IDs[7].expectedStatus = NoStatus // ns3 Service
	iteration0IDs[8].expectedStatus = NoStatus
	iteration0IDs[9].expectedStatus = NoStatus // ns4 Service
	iteration0IDs[10].expectedStatus = NoStatus
	testActions.addIteration(iteration0IDs, emptyIDs)

	/* iteration 1: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// run all the test actions
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

// TestNamespace3 tests:
// - kappnav ConfigMap with app-namespaces = ""
// - Application in ns4 with kappnav.component.namespaces = ""
// - Services/Deployments in ns1, ns2, ns3, ns4
// - Expected result:
//     Status returned for ns4 resources
//     No status returned for ns1, ns2, ns3 resources
func TestNamespace3(t *testing.T) {
	testName := "TestNamespace3"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		"Application": true,
		"Deployment":  true,
		"Service":     true,
	}

	// starting resources to pre-populate
	var files = []string{
		nsNolistkAppNavConfigMap,
		CrdApplication,
		ns4NoAnnoApplication,
		ns1Service,
		ns1Deployment,
		ns2Service,
		ns2Deployment,
		ns3Service,
		ns3Deployment,
		ns4Service,
		ns4Deployment,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: only ns4 resources should have status */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}

	iteration0IDs[3].expectedStatus = NoStatus // ns1 Service
	iteration0IDs[4].expectedStatus = NoStatus
	iteration0IDs[5].expectedStatus = NoStatus // ns2 Service
	iteration0IDs[6].expectedStatus = NoStatus
	iteration0IDs[7].expectedStatus = NoStatus // ns3 Service
	iteration0IDs[8].expectedStatus = NoStatus

	testActions.addIteration(iteration0IDs, emptyIDs)

	/* iteration 1: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// run all the test actions
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

// TestNamespace4 tests:
// - kappnav ConfigMap with app-namespaces = ""
// - Application in ns1 with kappnav.component.namespaces = "ns2, ns3"
// - Services/Deployments in ns1, ns2, ns3, ns4
// - Expected result:
//     Status returned for ns1, ns2, ns3 resources
//     No status returned for ns4 resources
func TestNamespace4(t *testing.T) {
	testName := "TestNamespace4"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		"Application": true,
		"Deployment":  true,
		"Service":     true,
	}

	// starting resources to pre-populate
	var files = []string{
		nsNolistkAppNavConfigMap,
		CrdApplication,
		ns1AnnoApplication,
		ns1Service,
		ns1Deployment,
		ns2Service,
		ns2Deployment,
		ns3Service,
		ns3Deployment,
		ns4Service,
		ns4Deployment,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: only ns1, ns2, ns3 resources should have status */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}

	iteration0IDs[9].expectedStatus = NoStatus // ns4 Service
	iteration0IDs[10].expectedStatus = NoStatus

	testActions.addIteration(iteration0IDs, emptyIDs)

	/* iteration 1: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// run all the test actions
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

/*
Test when adding application for the first time,
existing resource in other namespaces are not affected
*/
func TestNamespacePreExisting(t *testing.T) {
	testName := "TestNamespacePreExisting"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
		"Service":    true,
	}

	// starting resources to pre-populate
	var files = []string{
		/* 0 */ KappnavConfigFile,
		/* 1 */ CrdApplication,
		/* 2 */ ns1Service,
		/* 3 */ ns1Deployment,
		/* 4 */ ns2Service,
		/* 5 */ ns2Deployment,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: no applications. No resources should have status */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}

	// status should not be checked when there are not applications
	iteration0IDs[2].expectedStatus = NoStatus
	iteration0IDs[3].expectedStatus = NoStatus
	iteration0IDs[4].expectedStatus = NoStatus
	iteration0IDs[5].expectedStatus = NoStatus
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: add application to NS_1. All in NS_1 is normal.
	//  All in NS_2 remains NoStatus
	res, err := readOneResourceID(ns1App)
	if err != nil {
		t.Fatal(err)
	}
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs = append(iteration1IDs, res)
	arrayLength++
	iteration1IDs[2].expectedStatus = Normal
	iteration1IDs[3].expectedStatus = Normal
	iteration1IDs[6].expectedStatus = Normal
	testActions.addIteration(iteration1IDs, emptyIDs)

	/* iteration 4: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

/* Test adding kappnav.component.namepsaces annotation */
func TestComponentNamespaces(t *testing.T) {
	testName := "TestComponentNamespaces"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
		"Service":    true,
	}

	// starting resources to pre-populate
	var files = []string{
		/* 0 */ KappnavConfigFile,
		/* 1 */ CrdApplication,
		/* 2 */ ns1Service,
		/* 3 */ ns1Deployment,
		/* 4 */ ns1App,
		/* 5 */ ns2Service,
		/* 6 */ ns2Deployment,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: only namespace 1 resources should have status */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	iteration0IDs[5].expectedStatus = NoStatus // don't expect status to be checked for namespace 2
	iteration0IDs[6].expectedStatus = NoStatus // don't expect status to be checked for namespace 2
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: add annotation "kappnav.components.namespaces" : "ns2"
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[4].fileName = ns1aApp
	iteration1IDs[4].expectedStatus = Normal
	iteration1IDs[5].expectedStatus = Normal
	iteration1IDs[6].expectedStatus = Normal
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: Change the other namespace resource to Warning
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[4].expectedStatus = warning
	iteration2IDs[5].expectedStatus = warning
	testActions.addIteration(iteration2IDs, emptyIDs)

	// iteration 3: Change ns1 namespace resource to Problem
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	iteration3IDs[2].expectedStatus = problem
	iteration3IDs[4].expectedStatus = problem
	testActions.addIteration(iteration3IDs, emptyIDs)

	// iteration 4: Remove problem. Back to Warning.
	var iteration4IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration4IDs, iteration3IDs)
	iteration4IDs[2].expectedStatus = Normal
	iteration4IDs[4].expectedStatus = warning
	testActions.addIteration(iteration4IDs, emptyIDs)

	// iteration 5: Remove Warning . Back to Normal
	var iteration5IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration5IDs, iteration4IDs)
	iteration4IDs[4].expectedStatus = Normal
	iteration4IDs[5].expectedStatus = Normal
	testActions.addIteration(iteration5IDs, emptyIDs)

	/* clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNoNamespaceResource(t *testing.T) {
	testName := "TestNoNamespaceResource"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION: true,
		"Node":      true,
	}

	// starting resources to pre-populate
	var files = []string{
		/* 0 */ KappnavConfigFile,
		/* 1 */ CrdApplication,
		/* 2*/ nonsApp,
		/* 3 */ nonsNode,
	}
	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
	}

	/* Iteration 0: all normal */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1: Problem
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[3].expectedStatus = problem
	iteration1IDs[2].expectedStatus = problem
	testActions.addIteration(iteration1IDs, emptyIDs)

	// iteration 2: Warning
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration1IDs[3].expectedStatus = warning
	iteration1IDs[2].expectedStatus = warning
	testActions.addIteration(iteration2IDs, emptyIDs)

	// iteration 3: back to normal
	var iteration3IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration3IDs, iteration2IDs)
	iteration1IDs[3].expectedStatus = Normal
	iteration1IDs[2].expectedStatus = Normal
	testActions.addIteration(iteration3IDs, emptyIDs)

	/* iteration 4: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}
