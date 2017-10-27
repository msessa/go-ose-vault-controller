package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/vault/api"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func newAuthenticatedVaultClient() (*api.Client, error) {
	// TODO: authentication

	config := api.DefaultConfig()
	log.Debugf("connecting to vault api: %s", config.Address)
	c, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	self, err := c.Auth().Token().LookupSelf()
	if err != nil {
		return nil, err
	}
	log.Infof("authenticated to vault with token accessor %s", self.Data["accessor"])

	return c, err
}

func newAuthenticatedKubeInClusterClient() (*kubernetes.Clientset, error) {
	log.Debug("retrieving kubernetes in-cluster config")
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	log.Debugf("connecting to kubernetes api: %s", config.Host)
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	version, err := c.ServerVersion()
	if err != nil {
		return nil, err
	}
	log.Infof("successfully connected to kubernetes api %s", version.String())
	return c, err
}

func newAuthenticatedKubeClient(kubeconfig string) (*kubernetes.Clientset, error) {
	log.Debugf("using kubeconfig at %s", kubeconfig)
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	log.Debugf("connecting to kubernetes api: %s", config.Host)
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	version, err := c.ServerVersion()
	if err != nil {
		return nil, err
	}
	log.Infof("successfully connected to kubernetes api %s", version.String())
	return c, err
}
