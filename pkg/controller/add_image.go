package controller

import (
	"github.com/michaelgugino/htk-cluster-config-operator/pkg/controller/image"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, image.Add)
}
