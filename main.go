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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	routev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
)

const (
	kubeAPIURL = "http://localhost:9080"
)

var (
	apiURL        string        // URL of API server
	masterURL     string        // URL of Kube master
	kubeconfig    string        // path to kube config file. default <home>/.kube/config
	klogFlags     *flag.FlagSet // flagset for logging
	kubeClient    *kubernetes.Clientset
	routeV1Client *routev1.RouteV1Client
	isLatestOKD   bool = false
	isOKD         bool = false
	logger = NewLogger(false) //create new logger and set enableJSONLog false (log in plain text) 
)

func init() {
	// Print stacks and exit on SIGINT
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT)
		buf := make([]byte, 1<<20)
		<-sigChan
		stacklen := runtime.Stack(buf, true)
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen]))
		}
		os.Exit(1)
	}()
}

func main() {
	flag.Parse()

	var cfg *rest.Config
	var err error
	if strings.Compare(apiURL, "") != 0 {
		// running outside of Kube cluster
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("starting kappnav status controler outside cluster\n"))
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("masterURL: %s\n", masterURL))
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("kubeconfig: %s\n", kubeconfig))
		}
		cfg, err = clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("%s", err))
				os.Exit(255)
			}
		}
	} else {
		// running inside the Kube cluster
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, "starting kappnav status controler inside cluster\n")
		}
		apiURL = kubeAPIURL
		cfg, err = rest.InClusterConfig()
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("%s", err))
				os.Exit(255)
			}
		}
	}

	kubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("%s", err))
			os.Exit(255)
		}
	}

	var discClient = kubeClient.DiscoveryClient
	var dynamicClient dynamic.Interface
	dynamicClient, err = dynamic.NewForConfig(cfg)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("%s", err))
			os.Exit(255)
		}
	}

	kubeEnv := os.Getenv("KUBE_ENV")
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, "KUBE_ENV = "+kubeEnv)
	}
	if kubeEnv == "minishift" || kubeEnv == "okd" || kubeEnv == "ocp" {
		routeV1Client, err = routev1.NewForConfig(cfg)
		if err != nil {
			if logger.IsEnabled(LogTypeError) {
				logger.Log(CallerName(), LogTypeError, fmt.Sprintf("%s", err))
				os.Exit(255)
			}
		}
		groupLists, resourceLists, err := kubeClient.DiscoveryClient.ServerGroupsAndResources()
		if err == nil && groupLists != nil && resourceLists != nil {
			for _, resourceList := range resourceLists {
				for _, resource := range resourceList.APIResources {
					if logger.IsEnabled(LogTypeInfo) {
						logger.Log(CallerName(), LogTypeInfo, "Found resource kind = "+resource.Kind)
					}
					if resource.Kind == OpenShiftWebConsoleConfig {
						if logger.IsEnabled(LogTypeInfo) {
							logger.Log(CallerName(), LogTypeInfo, "Found resource kind OpenShiftWebConsoleConfig, assuming OpenShift Container Platform"+resource.Kind)
						}
						isLatestOKD = true
						break
					}
				}
			}
		}
	}

	plugin := &ControllerPlugin{dynamicClient, discClient, DefaultBatchDuration, calculateComponentStatus}

	//fetch kappnav CR and set init logging level before controller creates new cluster watcher to watch kappnav CR changes
	setInitLoggingData(plugin.dynamicClient)

	//create new cluster watcher
	_, err = NewClusterWatcher(plugin)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("%s", err))
			os.Exit(255)
		}
	}

	select {}
}

//fetch kappnav CR and set init logging level
func setInitLoggingData(dynInterf dynamic.Interface) {
	gvr := schema.GroupVersionResource{
		Group:    "kappnav.operator.kappnav.io",
		Version:  "v1",
		Resource: "kappnavs",
	}
	var intfNoNS = dynInterf.Resource(gvr)
	var intf dynamic.ResourceInterface
	// get interface from kappnav namespace
	intf = intfNoNS.Namespace(getkAppNavNamespace())

	// fetch the kappnav custom resource obj
	var unstructuredObj *unstructured.Unstructured
	var resName = "kappnav"
	unstructuredObj, _ = intf.Get(resName, metav1.GetOptions{})
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("kappnav CR obj %s ", unstructuredObj))
	}
	if unstructuredObj != nil {
		var objMap = unstructuredObj.Object
		if objMap != nil {
			tmp, _ := objMap[SPEC]
			spec := tmp.(map[string]interface{})
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("found kappnav CR spec %s ", spec))
			}
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
		}
	}
}

func printEvent(event watch.Event) {
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("event type %s, object type is %T\n", event.Type, event.Object))
	}
	printEventObject(event.Object, "    ")
}

func printEventObject(obj interface{}, indent string) {
	switch obj.(type) {
	case *unstructured.Unstructured:
		var unstructuredObj = obj.(*unstructured.Unstructured)
		// printObject(unstructuredObj.Object, indent)
		printUnstructuredJSON(unstructuredObj.Object, indent)
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, "\n")
		}
	default:
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%snot Unstructured: type: %T val: %s\n", indent, obj, obj))
		}
	}
}

func printUnstructuredJSON(obj interface{}, indent string) {
	data, err := json.MarshalIndent(obj, "", indent)
	if err != nil {
		if logger.IsEnabled(LogTypeError) {
			logger.Log(CallerName(), LogTypeError, fmt.Sprintf("JSON Marshaling failed %s", err))
			os.Exit(255)
		}
	}
	if logger.IsEnabled(LogTypeInfo) {
		logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%s\n", data))
	}
}

func printObject(obj interface{}, indent string) {
	nextIndent := indent + "    "
	switch obj.(type) {
	case int:
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%d", obj.(int)))
		}
	case bool:
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%t", obj.(bool)))
		}
	case float64:
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%f", obj.(float64)))
		}
	case string:
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%s", obj.(string)))
		}
	case []interface{}:
		var arr = obj.([]interface{})
		for index, elem := range arr {
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("\n%sindex:%d, type %T, ", indent, index, elem))
			}

			printObject(elem, nextIndent)
		}
	case map[string]interface{}:
		var objMap = obj.(map[string]interface{})
		for label, val := range objMap {
			if logger.IsEnabled(LogTypeInfo) {
				logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("\n%skey: %s type: %T| ", indent, label, val))
			}
			printObject(val, nextIndent)
		}
	default:
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("\n%stype: %T val: %s", indent, obj, obj))
		}
	}
}

func printPods(pods *corev1.PodList) {
	for _, pod := range pods.Items {
		if logger.IsEnabled(LogTypeInfo) {
			logger.Log(CallerName(), LogTypeInfo, fmt.Sprintf("%s", pod.ObjectMeta.Name))
		}
	}
}

func init() {
	// flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&apiURL, "apiURL", "", "The address of the kAppNav API server.")

	// init flags for klog
	klog.InitFlags(nil)
}

//SetLoggingLevel get logging level value from CR and reset the logger level
func SetLoggingLevel(loginfo string) {
	switch loginfo { 
	case "info":
		logger.SetLogLevel(LogLevelInfo)
		break
	case "debug":
		logger.SetLogLevel(LogLevelDebug)
		break
	case "error":
		logger.SetLogLevel(LogLevelError)
		break
	case "warning":
		logger.SetLogLevel(LogLevelWarning)
		break
	case "entry":
		logger.SetLogLevel(LogLevelEntry)
		break
	case "all":
		logger.SetLogLevel(LogLevelAll)
		break
	case "none":
		logger.SetLogLevel(LogLevelNone)
		break
	}	
}
