package pim

import (
	"errors"
	"testing"
)

func TestValidateActivateGroupArgs_MissingJustification(t *testing.T) {
	err := validateActivateGroupArgs(activateGroupArgs{Name: "g", Duration: 60}, true)
	if !errors.Is(err, errMissingFlag) {
		t.Fatalf("want errMissingFlag, got %v", err)
	}
}

func TestValidateActivateGroupArgs_MissingDuration(t *testing.T) {
	err := validateActivateGroupArgs(activateGroupArgs{Name: "g", Justification: "j"}, true)
	if !errors.Is(err, errMissingFlag) {
		t.Fatalf("want errMissingFlag, got %v", err)
	}
}

func TestValidateActivateGroupArgs_HappyPath(t *testing.T) {
	err := validateActivateGroupArgs(activateGroupArgs{Name: "g", Justification: "j", Duration: 60}, true)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
