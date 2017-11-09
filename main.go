package main

import (
	//"errors"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	vault "github.com/hashicorp/vault/api"
	"gopkg.in/alecthomas/kingpin.v2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var globals struct {
	clientset   *kubernetes.Clientset
	vaultclient *vault.Client
}

func podCreated(obj interface{}) {
	pod := obj.(*apiv1.Pod)

	if _, ok := pod.GetAnnotations()[fmt.Sprintf("%s/%s", *annotationPrefix, "vault-initialized")]; ok {
		log.Infof("pod '%s' already annotated", pod.Name)
		return
	}

	if dc, ok := pod.GetAnnotations()["openshift.io/deployment-config.name"]; ok {
		log.Infof("pod '%s' from DeploymentConfig '%s' created", pod.Name, dc)

		policyname, policybasepath, err := createVaultStandardPolicy(globals.vaultclient, *vaultMount, pod)
		if err != nil {
			log.Warnf("failed to create standard vault policy: %v.", err)
		}

		// tokenname := fmt.Sprintf("%s-%s", policyname, pod.Name)
		tokenpolicies := []string{policyname}

		// tk, err := createVaultOrphanToken(globals.vaultclient, tokenname, tokenpolicies)
		// if err != nil {
		// 	log.Errorf("failed to create vault token: %v.", err)
		// 	return
		// }

		// applyUpdate := func(updpod *apiv1.Pod, value string) {
		// 	if updpod.Annotations == nil {
		// 		updpod.Annotations = map[string]string{}
		// 	}
		// 	updpod.Annotations["mlctech.io/vault-token"] = value
		// 	updpod.Annotations["mlctech.io/vault-token-path"] = policypath
		// }
		roleID, secretID, err := createVaultAppRole(globals.vaultclient, fmt.Sprintf("%s-%s-%s", *vaultMount, pod.Namespace, dc), tokenpolicies)
		if err != nil {
			log.Errorf("failed to create vault approle: %v.", err)
			return
		}

		applyUpdate := func(updpod *apiv1.Pod, namespace string, annotations map[string]string) {
			if updpod.Annotations == nil {
				updpod.Annotations = map[string]string{}
			}
			for k, v := range annotations {
				updpod.Annotations[fmt.Sprintf("%s/%s", namespace, k)] = v
			}
		}

		log.Infof("annotating pod '%vs", pod.Name)
		annotations := map[string]string{
			"vault-role-id":     roleID,
			"vault-secret-id":   secretID,
			"vault-address":     globals.vaultclient.Address(),
			"vault-secret-path": policybasepath,
			"vault-initialized": "true",
		}
		if pod, err = updatePodWithRetries(pod.Namespace, pod, *annotationPrefix, annotations, applyUpdate); err != nil {
			log.Errorf("failed to annotate pod. %v.", err)
			return
		}

	} else {
		log.Debugf("pod '%s' not part of a deploymentConfig, skipping.", pod.Name)
	}

}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// This variables are set by linker flags externally
var (
	Version = "unknown"
	Build   = "dev"
)

// CLI Configuration
var (
	loglevel         = kingpin.Flag("loglevel", "Set logging level.").Short('l').Default("info").Enum("debug", "info", "warn", "crit", "panic")
	incluster        = kingpin.Flag("incluster", "Enable if program is being run inside kubernetes/openshift").Short('i').Bool()
	vaultMount       = kingpin.Flag("vaultpath", "Path on vault filesystem where secrets are located").Short('p').Required().OverrideDefaultFromEnvar("VAULTPATH").String()
	kubeconfig       = kingpin.Flag("kubeconfig", "Absolute path to the kubeconfig file").Default(filepath.Join(homeDir(), ".kube", "config")).String()
	selectednode     = kingpin.Flag("node", "Only act on pods scheduled on the specificed kubernetes node").Short('n').OverrideDefaultFromEnvar("NODESELECTOR").String()
	tlscert          = kingpin.Flag("tlscert", "TLS Client Certificate file for authentication").OverrideDefaultFromEnvar("TLSCERT").ExistingFile()
	tlskey           = kingpin.Flag("tlskey", "TLS Client Key file for authentication").OverrideDefaultFromEnvar("TLSKEY").ExistingFile()
	pkiauthpath      = kingpin.Flag("pkiauthpath", "Path of the PKI authentication backend on vault").Default("auth/cert").String()
	annotationPrefix = kingpin.Flag("prefix", "Prefix for Pod annotations").Default("mlctech.io").String()
)

func main() {
	var err error

	kingpin.Version(fmt.Sprintf("%s-%s", Version, Build))
	kingpin.Parse()

	// Configure logging
	ll, _ := log.ParseLevel(*loglevel)
	log.SetLevel(ll)

	vaultclient, renewer, err := newAuthenticatedVaultClient(*tlscert, *tlskey, *pkiauthpath)
	if err != nil {
		log.Fatalf("failed to create authenticated vault client: %v.", err)
		os.Exit(-1)
	}
	globals.vaultclient = vaultclient
	defer renewer.Stop()

	if *incluster {
		globals.clientset, err = newAuthenticatedKubeInClusterClient()
		if err != nil {
			log.Fatalf("failed to create authenticated kubernetes in-cluster client: %v.", err)
			os.Exit(-1)
		}
	} else {
		globals.clientset, err = newAuthenticatedKubeClient(*kubeconfig)
		if err != nil {
			log.Fatalf("failed to create authenticated kubernetes client: %v.", err)
			os.Exit(-1)
		}
	}

	//Create a cache to store Pods
	var podsStore cache.Store

	podsStore = watchForNewPods(globals.clientset, podsStore, *selectednode)

	// Loop forever
	for {
		select {
		case err := <-renewer.DoneCh():
			if err != nil {
				log.Fatalf("vault token cannot be further renewed. %v. exiting", err)
				break
			}

			// Renewal is now over
		case renewal := <-renewer.RenewCh():
			log.Debugf("Successfully renewed: %#v", renewal)
		}
	}

}
