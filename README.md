
### Grant the hostaccess SCC to the service account
```
oc adm policy add-scc-to-user hostmount-anyuid -z vault-controller
```

### Grant the `mlcl:vault-controller` role to the service account
```
oadm policy add-role-to-user mlcl:vault-controller -z vault-controller
```
