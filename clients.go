package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/vault/api"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func newAuthenticatedVaultClient() (*api.Client, *api.Renewer, error) {
	// TODO: authentication

	config := api.DefaultConfig()
	log.Debugf("connecting to vault api: %s", config.Address)
	c, err := api.NewClient(config)
	if err != nil {
		return nil, nil, err
	}

	lookup, err := c.Auth().Token().LookupSelf()
	if err != nil {
		return nil, nil, err
	}
	log.Infof("authenticated to vault with token accessor %s", lookup.Data["accessor"])

	if lookup.Data["renewable"].(bool) {
		log.Debugf("token is renewable. setting up renewer")
		// Token is renewable
		self, err := c.Auth().Token().RenewSelf(0)
		if err != nil {
			return nil, nil, err
		}
		renewer, err := c.NewRenewer(&api.RenewerInput{
			Secret: self,
		})
		go renewer.Renew()
		log.Infof("token renewer active")
		return c, renewer, nil
	}

	return c, nil, nil
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
