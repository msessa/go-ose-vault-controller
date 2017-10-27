#!/bin/bash -e

rm -rf target/
mkdir target/

echo "* Installing dep"
go get -u github.com/golang/dep/cmd/dep

echo "* Resolving dependecies"
dep ensure -v


