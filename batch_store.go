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
	"sync"
	"time"

	"k8s.io/klog"
)

/*
 The batchStore allows the producer to send resources changes on a
 closeable channel. Consumers of the resource calls the batchStore to
 get resource changes to be processed. Resource changes are
 batched for up to a configurable duration to reduce system
 resources usage during time of high activity.
 Resources that did not process successfully may be put back into
 batchStore for retry
*/

const (
	// DefaultBatchDuration - interval to batch resources before processing.
	// Defaults to 2 seconds as a compromise between responde time and resource usage
	DefaultBatchDuration = time.Second * 2
)

var (
	// error logger to limit execessive logging
	batchStoreErrorLogger = newSamplingLogger()
)

// resources to be processed in batches
type batchResources struct {
	applications    map[string]*resourceInfo
	nonApplications map[string]*resourceInfo
}

// Closeable channel to send resources.
// Once closed, sends are ignored
type resourceChannel struct {
	batchResourceChan chan *batchResources
	done              bool
	mutex             sync.Mutex
}

// Create a new closeable channel
func newResourceChannel() *resourceChannel {
	var resourceChannel = &resourceChannel{
		batchResourceChan: make(chan *batchResources, 1024),
		done:              false,
	}
	return resourceChannel
}

/*
  Close the channel
  Once closed, all future sends are ignored
*/
func (rc *resourceChannel) close() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if !rc.done {
		rc.done = true
		close(rc.batchResourceChan)
	}
}

// Send on the channel if not closed
func (rc *resourceChannel) send(resource *batchResources) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if !rc.done {
		rc.batchResourceChan <- resource
	}
}

/* Store resources that have changed for up to a given duration
   before making them available for processing
*/
type batchStore struct {
	batchDuration time.Duration   // how long to batch
	resController *ClusterWatcher // the cluster watcher

	timerStarted bool            // whether timer had started
	done         bool            // done  if no more resources to batch
	timerChan    chan struct{}   // timer channel to send timer event
	store        *batchResources // the actual resources to process

	mutex sync.Mutex
}

/* Create a new batchStore
resController: the cluster watcher
resChan:  channel to send applications that have changed
batchInterval: amount of time to batch resources before making them available for processing
*/
func newBatchStore(resController *ClusterWatcher, batchInterval time.Duration) *batchStore {
	ts := &batchStore{}
	ts.resController = resController
	ts.timerChan = make(chan struct{}, 1)
	ts.batchDuration = batchInterval
	ts.timerStarted = false
	ts.done = false
	ts.store = &batchResources{
		applications:    make(map[string]*resourceInfo),
		nonApplications: make(map[string]*resourceInfo),
	}
	return ts
}

/*
 start timer to wait for more resources to batch up
*/
func (ts *batchStore) startTimer() {
	if !ts.timerStarted {
		ts.timerStarted = true
		timerChan := ts.timerChan
		duration := ts.batchDuration
		go func() {
			time.Sleep(duration)
			timerChan <- struct{}{}
		}()
	}
}

/*
 * Get next batch of resources
 * Will block to batch the resources
 * Return:
 *     map of resources to process
 *     true if there resources to process, and false to shut down
 */
func (ts *batchStore) getNextBatch() (*batchResources, bool) {
	for {
		select {
		case resources, open := <-ts.resController.resourceChannel.batchResourceChan:
			// a new resource event
			ts.mutex.Lock()
			if !open {
				// channel closed
				if klog.V(4) {
					klog.Infof("batchStore.getNextBatch channel closed\n")
				}
				ts.done = true
				ts.mutex.Unlock()
				return nil, false
			}
			if klog.V(4) {
				klog.Infof("batchStore.getNextBatch received %d applications and %d resources\n", len(resources.applications), len(resources.nonApplications))
			}
			for _, resInfo := range resources.applications {
				ts.store.applications[resInfo.key()] = resInfo
			}
			for _, resInfo := range resources.nonApplications {
				ts.store.nonApplications[resInfo.key()] = resInfo
			}
			ts.startTimer()
			ts.mutex.Unlock()

		case <-ts.timerChan:
			ts.mutex.Lock()
			// If we are here, there is something in the store
			if klog.V(4) {
				klog.Infof("batchStore.getNextBatch timer popped applications %d, resources %d\n", len(ts.store.applications), len(ts.store.nonApplications))
			}
			if ts.done {
				ts.mutex.Unlock()
				return nil, false
			}
			ts.timerStarted = false // reset
			ret := ts.store
			ts.store = &batchResources{
				applications:    make(map[string]*resourceInfo),
				nonApplications: make(map[string]*resourceInfo),
			}
			ts.mutex.Unlock()
			return ret, true
		}
	}
}

// Put back resources to be retried again
// Check to ensure resources still exist before putting them back
func (ts *batchStore) putBack(resources *batchResources, putbackError error) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	if ts.done {
		return
	}
	batchStoreErrorLogger.logError(putbackError)
	numPutBack := 0

	for key, res := range resources.applications {
		if _, ok := ts.store.applications[key]; !ok {
			// not currently in the store
			_, exists, err := ts.resController.getResource(res.gvr, res.namespace, res.name)
			if err != nil {
				if klog.V(4) {
					klog.Errorf("Error getting resource %s %s %s from cache %s\n", res.gvr, res.namespace, res.name, err)
				}
			} else {
				if exists {
					// resource still exists. Put it back to be retried
					if klog.V(4) {
						klog.Infof("batchStore putting back %s %s %s to be retried\n", res.gvr, res.namespace, res.name)
					}
					ts.store.applications[key] = res
					numPutBack++
				} else {
					if klog.V(4) {
						klog.Infof("batchStor not putting back %s %s %s as it no longer exists\n", res.gvr, res.namespace, res.name)
					}
				}
			}
		}
	}
	for key, res := range resources.nonApplications {
		if _, ok := ts.store.nonApplications[key]; !ok {
			// not currently in the store
			_, exists, err := ts.resController.getResource(res.gvr, res.namespace, res.name)
			if err != nil {
				klog.Errorf("Error getting resource %s %s %s from cache %s\n", res.gvr, res.namespace, res.name, err)
			} else {
				if exists {
					// resource still exists. Put it back to be retried
					if klog.V(4) {
						klog.Infof("batchStore putting back %s %s %s to be retried\n", res.gvr, res.namespace, res.name)
					}
					ts.store.nonApplications[key] = res
					numPutBack++
				} else {
					if klog.V(4) {
						klog.Infof("batchStore.putBack: not putting back %s %s %s as it no longer exists\n", res.gvr, res.namespace, res.name)
					}
				}
			}
		}
	}
	// start timer
	// TODO: adjust timer based on frequency of error
	if numPutBack > 0 {
		ts.startTimer()
	}
}

/* To be run on a separate thread as the main entry point to
   process resources in the store
*/
func (ts *batchStore) run() {
	if klog.V(2) {
		klog.Infof("batchStore.run started\n")
	}
	for {
		if resources, ok := ts.getNextBatch(); ok {
			if err := processBatchOfApplicationsAndResources(ts, resources); err != nil {
				// put them back for retry later
				if klog.V(4) {
					klog.Errorf("Putting back resources due to error %s\n", err)
				}
				// TODO: can we put back only a subset
				ts.putBack(resources, err)
			}
		} else {
			// done
			break
		}
	}
	if klog.V(2) {
		klog.Infof("batchStore.run stopped\n")
	}
}
