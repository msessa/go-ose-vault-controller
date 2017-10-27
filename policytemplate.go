package main

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
