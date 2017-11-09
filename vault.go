package main

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	log "github.com/Sirupsen/logrus"
	vault "github.com/hashicorp/vault/api"
	apiv1 "k8s.io/api/core/v1"
)

type TemplateContext struct {
	Annotations map[string]string
	Labels      map[string]string
	Pod         apiv1.Pod
	Mountpoint  string
	Basepath    string
}

func createVaultStandardPolicy(client *vault.Client, vaultpath string, pod *apiv1.Pod) (string, string, error) {
	deploymentConfig := pod.GetAnnotations()["openshift.io/deployment-config.name"]
	policyname := fmt.Sprintf("%s-%s-%s", vaultpath, pod.Namespace, deploymentConfig)
	policybasepath := fmt.Sprintf("%s/transit/decrypt/%s-%s-%s", vaultpath, vaultpath, pod.Namespace, deploymentConfig)

	context := TemplateContext{
		Annotations: pod.GetAnnotations(),
		Labels:      pod.GetLabels(),
		Pod:         *pod,
		Mountpoint:  vaultpath,
		Basepath:    policybasepath,
	}
	// TODO: Cleanup
	tmpl, err := template.New("test").Parse(policytemplate)
	if err != nil {
		panic(err)
	}
	var doc bytes.Buffer
	err = tmpl.Execute(&doc, context)
	//policyrules := fmt.Sprintf(policytemplate, vaultpath, pod.Namespace, deploymentConfig)
	policyrules := doc.String()

	if policycontent, err := client.Sys().GetPolicy(policyname); err != nil || policycontent == "" {
		log.Debugf("creating standard vault policy: %s", policyname)
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

func createVaultAppRole(client *vault.Client, roleName string, policies []string) (string, string, error) {

	dcRole, err := client.Logical().Read(fmt.Sprintf("auth/approle/role/%s", roleName))
	if err != nil {
		return "", "", err
	}
	if dcRole == nil {
		log.Debugf("creating vault approle '%s'", roleName)

		roleconfig := map[string]interface{}{
			"period":             "1h",
			"secret_id_num_uses": "1",
			"secret_id_ttl":      "5m",
			"policies":           strings.Join(policies, ","),
		}
		_, err = client.Logical().Write(fmt.Sprintf("auth/approle/role/%s", roleName), roleconfig)
		if err != nil {
			return "", "", err
		}
		log.Infof("successfuly created new vault approle '%s'", roleName)
	}
	log.Debugf("retrieving approle %s role-id", roleName)
	roleID, err := client.Logical().Read(fmt.Sprintf("auth/approle/role/%s/role-id", roleName))
	if err != nil {
		return "", "", err
	}
	log.Debugf("role-id is %s", roleID.Data["role_id"])

	log.Debugf("creating new approle %s secret-id", roleName)
	secretID, err := client.Logical().Write(fmt.Sprintf("auth/approle/role/%s/secret-id", roleName), map[string]interface{}{})
	if err != nil {
		return "", "", err
	}
	log.Infof("created new secret-id with accessor %s for role %v", secretID.Data["secret_id_accessor"], roleName)
	return roleID.Data["role_id"].(string), secretID.Data["secret_id"].(string), nil
}
