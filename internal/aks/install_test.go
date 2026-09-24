package aks

import "testing"

func TestKubeVersionRe(t *testing.T) {
	if !kubeVersionRe.MatchString("v1.31.2") {
		t.Error("expected v1.31.2 to match")
	}
	if kubeVersionRe.MatchString("v1.31.2 -o /etc/x") {
		t.Error("expected injected string to be rejected")
	}
	if kubeVersionRe.MatchString("") {
		t.Error("expected empty string to be rejected")
	}
}
