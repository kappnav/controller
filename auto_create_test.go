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
	"testing"
)

const (
	autocreateDeploymentDefault = "test_data/autoDeployment0.json"
	autocreateDeployment1       = "test_data/autoDeployment1.json"
	autocreateDeployment2       = "test_data/autoDeployment2.json"
	autocreateDeployment3       = "test_data/autoDeployment3.json"
	autocreateDeployment4       = "test_data/autoDeployment4.json"
	autocreateDeployment4A      = "test_data/autoDeployment4a.json"
	autocreateDeployment5       = "test_data/autoDeployment5.json"
	autocreateDeployment5A      = "test_data/autoDeployment5a.json"
	autocreateDeployment5B      = "test_data/autoDeployment5b.json"
	autocreateDeployment6       = "test_data/autoDeployment6.json"
	autocreateDeployment6A      = "test_data/autoDeployment6a.json"
	autocreateDeployment6B      = "test_data/autoDeployment6b.json"
	autocreateDeployment7       = "test_data/autoDeployment7.json"
	autocreateDeployment8       = "test_data/autoDeployment8.json"
	autocreateDeployment9       = "test_data/autoDeployment9.json"
	autocreateDeployment7p      = "test_data/autoDeployment7p.json"
	autocreateDeployment8p      = "test_data/autoDeployment8p.json"
	autocreateDeployment9p      = "test_data/autoDeployment9p.json"
	autocreateDeployment7b      = "test_data/autoDeployment7b.json"
	autocreateDeployment8b      = "test_data/autoDeployment8b.json"
	autocreateDeployment9b      = "test_data/autoDeployment9b.json"
	autocreateDeployment7bd     = "test_data/autoDeployment7bd.json"
	autocreateDeployment8bd     = "test_data/autoDeployment8bd.json"
	autocreateDeployment9bd     = "test_data/autoDeployment9bd.json"

	autocreateAppDefault = "test_data/autoApp0.json"
	autocreateApp1       = "test_data/autoApp1.json"
	autocreateApp2       = "test_data/autoApp2.json"
	autocreateApp3       = "test_data/autoApp3.json"
	autocreateApp4       = "test_data/autoApp4.json"
	autocreateApp5       = "test_data/autoApp5.json"
	autocreateApp5A      = "test_data/autoApp5a.json"
	autocreateApp5B      = "test_data/autoApp5b.json"
	autocreateApp6       = "test_data/autoApp6.json"
	autocreateApp6A      = "test_data/autoApp6a.json"
	autocreateApp6B      = "test_data/autoApp6b.json"
	autocreateApp7       = "test_data/autoApp7.json"
	autocreateApp7p      = "test_data/autoApp7p.json"
	autocreateApp7b      = "test_data/autoApp7b.json"
	autocreateApp7bd     = "test_data/autoApp7bd.json"
	
	autocreateStatefulset = "test_data/autoStatefulSet1.json"
	autocreateAppStateful1 = "test_data/autoAppStateful1.json"
)

var testStringToArrayOfAlphaNumericData = map[string][]string{
	"a, b, c":                       []string{"a", "b", "c"},
	"a, b, c\n":                     []string{"a", "b", "c"},
	"\n\ra, \rb\t, \nc\r":           []string{"a", "b", "c"},
	"a, b c, d\re, f\ng, h i\n":     []string{"a"},
	"a\r z, b c, d\re, f\ng, h i\n": []string{},
}

func TestStringToArrayOfAlphaNumeric(t *testing.T) {
	testName := "TestStringToArrayOfAlphaNumeric"
	for key, val := range testStringToArrayOfAlphaNumericData {
		ret := stringToArrayOfAlphaNumeric(key)
		if !sameStringArray(val, ret) {
			t.Fatal(fmt.Errorf("test %s failed: input: %s, output: %s, expected: %s", testName, key, ret, val))
		}
	}
}

/* Test auto create with default values
 */
func autoCreateTestHelper(t *testing.T, testName string, kindsToCheckStatus map[string]bool, files []string, autoCreatedFiles []string) {

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	iteration0AutoCreatedIDs, err := readResourceIDs(autoCreatedFiles)
	if err != nil {
		t.Fatal(err)
		return
	}

	/* Iteration 0: create resources, and check for auto-created */
	testActions := newTestActions(testName, kindsToCheckStatus)
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	/* iteration 1: clean up. Delete resources, and check auto-created are deleted */
	var emptyIDs = []resourceID{}
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
		return
	}
}

/* Test auto create with default values
 */
func TestAutoCreateDefault(t *testing.T) {
	testName := "TestAutoCreateDefault"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ CrdApplication,
		/* 1 */ KappnavConfigFile,
		/* 2 */ autocreateDeploymentDefault,
	}

	var autoCreatedFiles = []string{
		/* 0 */ autocreateAppDefault,
	}

	autoCreateTestHelper(t, testName, kindsToCheckStatus, files, autoCreatedFiles)
}

/* Test auto create with no default values
 */
func TestAutoCreateNonDefault1(t *testing.T) {
	testName := "TestAutoCreateNonDefault1"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ CrdApplication,
		/* 1 */ KappnavConfigFile,
		/* 2 */ autocreateDeployment1,
	}

	var autoCreatedFiles = []string{
		/* 0 */ autocreateApp1,
	}

	autoCreateTestHelper(t, testName, kindsToCheckStatus, files, autoCreatedFiles)

}

func TestAutoCreateNonDefault2(t *testing.T) {
	testName := "TestAutoCreateNonDefault2"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
	}

	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment2,
	}

	var autoCreatedFiles = []string{
		autocreateApp2,
	}
	autoCreateTestHelper(t, testName, kindsToCheckStatus, files, autoCreatedFiles)

}

func TestAutoCreateNonDefault3(t *testing.T) {
	testName := "TestAutoCreateNonDefault3"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
	}

	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment3,
	}

	var autoCreatedFiles = []string{
		autocreateApp3,
	}
	autoCreateTestHelper(t, testName, kindsToCheckStatus, files, autoCreatedFiles)

}

func TestAutoCreateChangleLabel(t *testing.T) {
	testName := "TestAutoCreateChangeLabel"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
	}

	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment4,
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	/* Iteration 0: all normal. No application is created */
	testActions := newTestActions(testName, kindsToCheckStatus)
	var emptyIDs = []resourceID{}
	iteration0IDs[2].expectedStatus = NoStatus
	testActions.addIteration(iteration0IDs, emptyIDs)

	// iteration 1:  change kappnav.app.auto-create  to "true"
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[2].fileName = autocreateDeployment4A
	iteration1IDs[2].expectedStatus = Normal

	var autoCreatedFiles = []string{
		autocreateApp4,
	}
	iteration1AutoCreatedIDs, err := readResourceIDs(autoCreatedFiles)
	if err != nil {
		t.Fatal(err)
		return
	}
	testActions.addIteration(iteration1IDs, iteration1AutoCreatedIDs)

	/* Iteation 2: Switch back to no auto-create. The auto-created app should be auto-deleted  */
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[2].fileName = autocreateDeployment4
	testActions.addIteration(iteration2IDs, emptyIDs)

	/* iteration 3: clean up */
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAutoCreateChangeName(t *testing.T) {
	testName := "TestAutoCreateChangeName"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
	}

	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment5,
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	iteration0AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp5})
	if err != nil {
		t.Fatal(err)
		return
	}

	/* Iteration 0: Create default applications. */
	testActions := newTestActions(testName, kindsToCheckStatus)
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	// iteration 1:  add kappnav.app.auto-create.name
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[2].fileName = autocreateDeployment5A

	iteration1AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp5A})
	if err != nil {
		t.Fatal(err)
		return
	}
	testActions.addIteration(iteration1IDs, iteration1AutoCreatedIDs)

	/* Iteation 2: change kappnav.app.auto-create.name */
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[2].fileName = autocreateDeployment5B
	iteration2AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp5B})
	if err != nil {
		t.Fatal(err)
		return
	}
	testActions.addIteration(iteration2IDs, iteration2AutoCreatedIDs)

	/* Iteation 3: revert back to original */
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	/* iteration 4: clean up */
	var emptyIDs = []resourceID{}
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAutoCreateChangeAnnotation(t *testing.T) {
	testName := "TestAutoCreateChangeAnnotation"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
	}

	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment6,
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	iteration0AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp6})
	if err != nil {
		t.Fatal(err)
		return
	}

	/* Iteration 0: Create default applications. */
	testActions := newTestActions(testName, kindsToCheckStatus)
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	/* iteration 1 */
	arrayLength := len(iteration0IDs)
	var iteration1IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration1IDs, iteration0IDs)
	iteration1IDs[2].fileName = autocreateDeployment6A

	iteration1AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp6A})
	if err != nil {
		t.Fatal(err)
		return
	}
	testActions.addIteration(iteration1IDs, iteration1AutoCreatedIDs)

	/* iteration 6 */
	var iteration2IDs = make([]resourceID, arrayLength, arrayLength)
	copy(iteration2IDs, iteration1IDs)
	iteration2IDs[2].fileName = autocreateDeployment6B
	iteration2AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp6B})
	if err != nil {
		t.Fatal(err)
		return
	}
	testActions.addIteration(iteration2IDs, iteration2AutoCreatedIDs)

	/* Iteation 3: revert back to original */
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	/* iteration 4: clean up */
	var emptyIDs = []resourceID{}
	testActions.addIteration(emptyIDs, emptyIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

	// make all trasition of testAction
	err = testActions.transitionAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAutoCreateDeleteOrphan(t *testing.T) {
	testName := "TestAutoCreateDeleteOrphan"
	beforeTest()
	var kindsToCheckStatus = map[string]bool{
		APPLICATION:  true,
		"Deployment": true,
	}

	var files = []string{
		autocreateDeployment1, // must be a different deployment from that which created autocreateAppDefault
		CrdApplication,
		KappnavConfigFile,
		autocreateAppDefault,
	}

	resources, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	testActions := newTestActions(testName, kindsToCheckStatus)
	clusterWatcher, err := createClusterWatcher(resources, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

	err = waitForAutoDelete(testName, 0, clusterWatcher, resources[3])
	if err != nil {
		t.Fatal(err)
	}
}

/* Test auto create with StatefulSet
 */
func TestAutoCreateStatefulSet(t *testing.T) {
	testName := "TestAutoCreateStatefulSet"
	beforeTest()
	// kinds to check for status
	var kindsToCheckStatus = map[string]bool{
		APPLICATION: true,
		STATEFULSET: true,
	}

	// resources to pre-populate
	var files = []string{
		/* 0 */ CrdApplication,
		/* 1 */ KappnavConfigFile,
		/* 2 */ autocreateStatefulset}

	var autoCreatedFiles = []string{
		/* 0 */ autocreateAppStateful1,
	}

	autoCreateTestHelper(t, testName, kindsToCheckStatus, files, autoCreatedFiles)
}

/* Test auto create when multiple deployments contain autoCreateName associate to same app
 */
 func TestAutoCreateMultipleDeployments(t *testing.T) {
    testName := "TestAutoCreateMultipleDeployments"
	beforeTest()
	
    // kinds to check for status
    var kindsToCheckStatus = map[string]bool{
        APPLICATION:  true,
        "Deployment": true,
	}
	
	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment7,	
		autocreateDeployment8,	
		autocreateDeployment9,	
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	// auto-create app 
	iteration0AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp7})
    if err != nil {
        t.Fatal(err)
        return
	}

	/* Iteration 0: auto-create application is created */
	testActions := newTestActions(testName, kindsToCheckStatus)
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	// resources to be kept 
	var files1 = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment9,
    }

	iteration1IDs, err := readResourceIDs(files1)
    if err != nil {
        t.Fatal(err)
        return
    }

	/* Iteration 1: delete deployments auto7 and auto8 */
	var emptyIDs = []resourceID{}
	iteration0IDs[2].expectedStatus = NoStatus
	testActions.addIteration(iteration1IDs, emptyIDs)

	// verify if auto-create app still exists after 2 deployments are deleted
	testActions.addIteration(iteration1IDs, iteration0AutoCreatedIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

    err = testActions.transitionAll()
    if err != nil {
		// wait for auto7 to be deleted
		err = waitForAutoDelete(testName, 0, clusterWatcher, iteration0IDs[2])
		if err != nil {
			t.Fatal(err)
		}
		// wait for auto8 to be deleted
		err = waitForAutoDelete(testName, 0, clusterWatcher, iteration0IDs[3])
		if err != nil {
			t.Fatal(err)
		}
    }
}

/* Test auto create when multiple deployments contain part-of label assoicate to same app
 */
 func TestAutoCreatePartOfLabelMultipleDeployments(t *testing.T) {
    testName := "TestAutoCreatePartOfLabelMultipleDeployments"
	beforeTest()
	
    // kinds to check for status
    var kindsToCheckStatus = map[string]bool{
        APPLICATION:  true,
        "Deployment": true,
	}
	
	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment7p,	
		autocreateDeployment8p,	
		autocreateDeployment9p,	
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	// auto-create app 
	iteration0AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp7p})
    if err != nil {
        t.Fatal(err)
        return
	}

	/* Iteration 0: auto-create auto7p-app application is created */
	testActions := newTestActions(testName, kindsToCheckStatus)
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	// resources to be kept 
	var files1 = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment9p,
    }

	iteration1IDs, err := readResourceIDs(files1)
    if err != nil {
        t.Fatal(err)
        return
    }

	/* Iteration 1: delete deployments auto7p and auto8p */
	var emptyIDs = []resourceID{}
	iteration0IDs[2].expectedStatus = NoStatus
	testActions.addIteration(iteration1IDs, emptyIDs)

	// verify if auto-create app still exists after 2 deployments are deleted
	testActions.addIteration(iteration1IDs, iteration0AutoCreatedIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

	err = testActions.transitionAll()
	
    if err != nil {
		// wait for auto7p to be deleted		
		err = waitForAutoDelete(testName, 0, clusterWatcher, iteration0IDs[2])
		if err != nil {
			t.Fatal(err)
		}
		// wait for auto8p to be deleted
		err = waitForAutoDelete(testName, 0, clusterWatcher, iteration0IDs[3])
		if err != nil {
			t.Fatal(err)
		}
    }
}

/* Test auto create when multiple deployments containin both part-of label and autoCreateName associate to same app
 */
 func TestAutoCreatePartOfLabelAndAutoCreateNameMultipleDeployments(t *testing.T) {
    testName := "TestAutoCreatePartOfLabelAndAutoCreateNameMultipleDeployments"
	beforeTest()
	
    // kinds to check for status
    var kindsToCheckStatus = map[string]bool{
        APPLICATION:  true,
        "Deployment": true,
	}
	
	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment7b,	
		autocreateDeployment8b,	
		autocreateDeployment9b,	
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	// auto-create app 
	iteration0AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp7b})
    if err != nil {
        t.Fatal(err)
        return
	}

	/* Iteration 0: auto-create auto7b-app application is created */
	testActions := newTestActions(testName, kindsToCheckStatus)
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	// resources to be kept 
	var files1 = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment9b,
    }

	iteration1IDs, err := readResourceIDs(files1)
    if err != nil {
        t.Fatal(err)
        return
    }

	/* Iteration 1: delete deployments auto7b and auto8b */
	var emptyIDs = []resourceID{}
	iteration0IDs[2].expectedStatus = NoStatus
	testActions.addIteration(iteration1IDs, emptyIDs)

	// verify if auto-create app still exists after 2 deployments are deleted
	testActions.addIteration(iteration1IDs, iteration0AutoCreatedIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

	err = testActions.transitionAll()
	
    if err != nil {
		// wait for auto7b to be deleted		
		err = waitForAutoDelete(testName, 0, clusterWatcher, iteration0IDs[2])
		if err != nil {
			t.Fatal(err)
		}
		// wait for auto8b to be deleted
		err = waitForAutoDelete(testName, 0, clusterWatcher, iteration0IDs[3])
		if err != nil {
			t.Fatal(err)
		}
    }
}

/* Test auto create when multiple deployments containin both part-of label and autoCreateName associate to different apps
 */
 func TestAutoCreatePartOfLabelAndAutoCreateNameWithDiffApps(t *testing.T) {
    testName := "TestAutoCreatePartOfAutoCreateNameWithDiffAppsMultipleDeployments"
	beforeTest()
	
    // kinds to check for status
    var kindsToCheckStatus = map[string]bool{
        APPLICATION:  true,
        "Deployment": true,
	}
	
	// resources to pre-populate
	var files = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment7bd,	
		autocreateDeployment8bd,	
		autocreateDeployment9bd,	
	}

	iteration0IDs, err := readResourceIDs(files)
	if err != nil {
		t.Fatal(err)
		return
	}

	// auto-create app 
	iteration0AutoCreatedIDs, err := readResourceIDs([]string{autocreateApp7bd})
    if err != nil {
        t.Fatal(err)
        return
	}

	/* Iteration 0: auto-create auto7bd-app and auto7bdf-app applications are created */
	testActions := newTestActions(testName, kindsToCheckStatus)
	testActions.addIteration(iteration0IDs, iteration0AutoCreatedIDs)

	// resources to be kept 
	var files1 = []string{
		CrdApplication,
		KappnavConfigFile,
		autocreateDeployment9bd,
    }

	iteration1IDs, err := readResourceIDs(files1)
    if err != nil {
        t.Fatal(err)
        return
    }

	/* Iteration 1: delete deployments auto7bd and auto8bd */
	var emptyIDs = []resourceID{}
	iteration0IDs[2].expectedStatus = NoStatus
	testActions.addIteration(iteration1IDs, emptyIDs)

	// verify if auto-create auto7bd-app app still exists after 2 deployments are deleted
	testActions.addIteration(iteration1IDs, iteration0AutoCreatedIDs)

	clusterWatcher, err := createClusterWatcher(iteration0IDs, testActions, StatusFailureRate)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer clusterWatcher.shutDown()

	err = testActions.transitionAll()
	
    if err != nil {
		// wait for auto7bd to be deleted		
		err = waitForAutoDelete(testName, 0, clusterWatcher, iteration0IDs[2])
		if err != nil {
			t.Fatal(err)
		}
		// wait for auto8bd to be deleted
		err = waitForAutoDelete(testName, 0, clusterWatcher, iteration0IDs[3])
		if err != nil {
			t.Fatal(err)
		}
    }
}

