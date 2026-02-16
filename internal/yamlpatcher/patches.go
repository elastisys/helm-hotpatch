package yamlpatcher

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

type Patches struct {
	Add    []*unstructured.Unstructured
	Patch  []*unstructured.Unstructured
	Remove []*unstructured.Unstructured
}
