package main

// %[1]s = Vault Secret Mount
// %[2]s = OSE Namespace
// %[3]s = OSE DeploymentConfig

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
		  },
		  "%[1]s/%[2]s/%[3]s": {
			"capabilities": [
		  		"read"
			]
	  	}
	}
}
`
