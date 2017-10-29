package main

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	vault "github.com/hashicorp/vault/api"
)

func createVaultStandardPolicy(client *vault.Client, vaultpath string, namespace string, deploymentConfig string) (string, string, error) {
	policyname := fmt.Sprintf("%s-%s-%s", vaultpath, namespace, deploymentConfig)
	policybasepath := fmt.Sprintf("%s/%s/%s", vaultpath, namespace, deploymentConfig)
	policyrules := fmt.Sprintf(policytemplate, vaultpath, namespace, deploymentConfig)

	if policycontent, err := client.Sys().GetPolicy(policyname); err != nil || policycontent == "" {
		log.Debugf("creating standrd vault policy: %s", policyname)
		err := client.Sys().PutPolicy(policyname, policyrules)
		if err != nil {
			return "", "", err
		}
		log.Infof("created standard vault policy %s", policyname)
	} else {
		log.Debug("vault standard policy already exists, skipping")
	}
	return policyname, policybasepath, nil
}

func createVaultOrphanToken(client *vault.Client, tokenName string, policies []string) (*vault.Secret, error) {
	log.Debugf("creating wrapped vault token '%s'", tokenName)
	cr := &vault.TokenCreateRequest{
		Policies:        policies,
		NoParent:        true,
		NoDefaultPolicy: false,
		DisplayName:     tokenName,
	}

	tk, err := client.Auth().Token().CreateOrphan(cr)
	if err != nil {
		return nil, err
	}
	log.Infof("created vault token. accessor: %s ", tk.Auth.Accessor)
	return tk, nil
}
