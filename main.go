package main

import (
	//"errors"
	"time"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	//"k8s.io/apimachinery/pkg/api/errors"
	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	vault "github.com/hashicorp/vault/api"
)

var policytemplate = `
{
	"path": {
		"%[1]s/%[2]s/%[3]s/": {
			"capabilities": [
		  		"list"
			]
	  	},
	  	"%[1]s/%[2]s/%[3]s": {
			"capabilities": [
		  		"read"
			]
	  	}
	}
}
`

var KVMOUNT = "osenp"

var globals struct {
	clientset *kubernetes.Clientset
	podsStore cache.Store
	vaultclient *vault.Client
}

func podCreated(obj interface{}) {
	pod := obj.(*v1.Pod)
	
	if _, ok := pod.GetAnnotations()["mlctech.io/vault-token"]; ok {
		log.Infof("pod '%s' already annotated", pod.Name)
		return
	}

	if dc, ok := pod.GetAnnotations()["openshift.io/deployment-config.name"]; ok {
		log.Infof("pod '%s' from DeploymentConfig '%s' created", pod.Name, dc)

		data := make(map[string]interface{})
		policy := fmt.Sprintf(policytemplate, KVMOUNT, pod.Namespace, dc)
		data["rules"] = policy

		policyname := fmt.Sprintf("%s-%s-%s", KVMOUNT, pod.Namespace, dc)

		log.Infof("creating vault policy '%s'", policyname)
		err := globals.vaultclient.Sys().PutPolicy(policyname, policy)
		//err := globals.vaultclient.Logical().Write(policypath, data)
		//r := globals.vaultclient.NewRequest()
		if err != nil {
			log.Errorf("failed to create vault policy: %v.", err)
			return
		}
		log.Infof("vault policy created")

		tokenname := fmt.Sprintf("%s-%s", policyname, pod.Name)
		log.Infof("creating wrapped vault token '%s'", tokenname)
		cr := &vault.TokenCreateRequest{
			Policies:        []string{"default", policyname},
			NoParent:        true,
			NoDefaultPolicy: false,
			DisplayName:     tokenname,
		}


		tk, err := globals.vaultclient.Auth().Token().CreateOrphan(cr)
		if err != nil {
			log.Errorf("failed to create vault token: %v.", err)
			return
		}
		log.Infof("got token accessor '%vs", tk.Auth.Accessor)

		log.Infof("annotating pod '%s'", pod.Name)
		annotations := pod.GetAnnotations()
		annotations["mlctech.io/vault-token"] = tk.Auth.ClientToken
		pod.SetAnnotations(annotations)

		apod := pod
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
			apod, err = globals.clientset.Pods(pod.Namespace).Update(apod)
			return
		})
		if err != nil {
			log.Errorf("failed to annotate pod. %v.", err)
			return
		}

		
		// for {
		// _, err = globals.clientset.Pods(pod.Namespace).Update(apod);
		// if errors.IsConflict(err) {
		// 	// Deployment is modified in the meanwhile, query the latest version
		// 	// and modify the retrieved object.
		// 	log.Warnf("encountered conflict, retrying")
		// 	apod, err := globals.clientset.Pods(apod.Namespace).Get(apod.Name)
		// 	if err != nil {
		// 		panic(fmt.Errorf("get failed: %+v", err))
		// 	}
		// } else if err != nil {
		// 	log.Errorf("failed to annotate pod. %v.", err)
		// 	return
		// } else {
		// 	break
		// }
		// }
	} else {
		log.Debugf("pod '%s' not part of a deploymentConfig, skipping.", pod.Name)
	}
	
}

func podDeleted(obj interface{}) {
    pod := obj.(*v1.Pod)
    fmt.Println("Pod deleted: "+pod.ObjectMeta.Name)
}

func watchPods(store cache.Store) cache.Store {
	//Define what we want to look for (Pods)
	watchlist := cache.NewListWatchFromClient(globals.clientset.Core().RESTClient(), "pods", v1.NamespaceAll, fields.Everything())

	resyncPeriod := 30 * time.Minute

	//Setup an informer to call functions when the watchlist changes
	eStore, eController := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    podCreated,
			DeleteFunc: podDeleted,
		},
	)

	//Run the controller as a goroutine
	stop := make(chan struct{})
	go eController.Run(stop)
	return eStore
}

func main() {

	vc, err := vault.NewClient(vault.DefaultConfig())
	if err != nil {
		log.Errorf("failed to create vault client: %v.", err)
		panic(err)
	}

	vtself, err := vc.Auth().Token().LookupSelf()
	if err != nil {
		log.Errorf("failed to verify vault token: %v.", err)
		panic(err)
	}
	log.Infof("Authenticated with token accessor %s", vtself.Data["accessor"])

	globals.vaultclient = vc

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Warnf("failed to create in-cluster client: %v.", err)
		config, err = clientcmd.BuildConfigFromFlags("", "/Users/msessa/.kube/config")
		if err != nil {
			log.Warnf("failed to create kubeconfig client: %v.", err)
		}
	}
	log.Info("connecting to kubernetes api: ", config.Host)
	globals.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}



	version, err := globals.clientset.ServerVersion()
	if err != nil {
		panic(err)
	}
	log.Infof("successfully connected to kubernetes api %s", version.String())

	//Create a cache to store Pods
	

	globals.podsStore = watchPods(globals.podsStore)
	select{ }

	
}