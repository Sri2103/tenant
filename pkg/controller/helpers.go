package controller

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

func resourceQuantity(val string) resource.Quantity {
	if val == "" {
		return resource.MustParse("0")
	}
	return resource.MustParse(val)
}

func ignoreAlreadyExists(err error) error {
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
