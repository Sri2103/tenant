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

func containsString(slice []string, b string) bool {
	for _, v := range slice {
		if v == b {
			return true
		}
	}
	return false
}

func removeString(slice []string, b string) []string {
	out := []string{}
	for _, v := range slice {
		if v == b {
			continue
		}
		out = append(out, v)
	}
	return out
}
