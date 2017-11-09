package main

// %[1]s = Vault Secret Mount
// %[2]s = OSE Namespace
// %[3]s = OSE DeploymentConfig

var policytemplate = `
{
	"path": {
		"{{ .Basepath }}": {
			"capabilities": [
		  		"update"
			]
	  	}
	}
}
`
