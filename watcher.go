package main

import (
	"time"

	log "github.com/Sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func watchForNewPods(client *kubernetes.Clientset, store cache.Store, selectednode string) cache.Store {

	// Sets up a filter for pods running on the node specified by the 'node' flag
	var selector = fields.Everything()
	if selectednode != "" {
		log.Debugf("node selector enabled. limiting actions to pods on node %v", selectednode)
		selector = fields.Set{"spec.nodeName": selectednode}.AsSelector()
	}

	//Define what we want to look for (Pods)
	watchlist := cache.NewListWatchFromClient(client.Core().RESTClient(), "pods", apiv1.NamespaceAll, selector)

	resyncPeriod := 30 * time.Minute

	//Setup an informer to call functions when the watchlist changes
	eStore, eController := cache.NewInformer(
		watchlist,
		&apiv1.Pod{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: podCreated,
		},
	)

	//Run the controller as a goroutine
	stop := make(chan struct{})
	go eController.Run(stop)
	return eStore
}

type updatePodFunc func(controller *apiv1.Pod, prefix string, annotations map[string]string)

// updatePodWithRetries retries updating the given pod on conflict with the following steps:
// 1. Get latest resource
// 2. applyUpdate
// 3. Update the resource
func updatePodWithRetries(namespace string, pod *apiv1.Pod, prefix string, annotations map[string]string, applyUpdate updatePodFunc) (*apiv1.Pod, error) {
	// Deep copy the pod in case we failed on Get during retry loop
	oldPod := pod.DeepCopy()
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (e error) {
		// Apply the update, then attempt to push it to the apiserver.

		applyUpdate(pod, prefix, annotations)

		if pod, e = globals.clientset.CoreV1().Pods(namespace).Update(pod); e == nil {
			return
		}
		updateErr := e
		if pod, e = globals.clientset.CoreV1().Pods(namespace).Get(oldPod.Name, metav1.GetOptions{}); e != nil {
			pod = oldPod
		}
		// Only return the error from update
		return updateErr
	})
	// If the error is non-nil the returned pod cannot be trusted, if it is nil, the returned
	// controller contains the applied update.
	return pod, err
}
