package pim

import (
	"errors"
	"testing"
)

func TestValidateActivateResourceArgs_MissingTicket(t *testing.T) {
	err := validateActivateResourceArgs(activateResourceArgs{
		Role: "Contributor", Scope: "/subscriptions/x",
		Justification: "j", Duration: 60,
	}, true /* noInput */)
	if !errors.Is(err, errMissingFlag) {
		t.Fatalf("want errMissingFlag, got %v", err)
	}
}

func TestValidateActivateResourceArgs_MissingDuration(t *testing.T) {
	err := validateActivateResourceArgs(activateResourceArgs{
		Role: "Contributor", Scope: "/subscriptions/x",
		Ticket: "Jira:1", Justification: "j",
	}, true)
	if !errors.Is(err, errMissingFlag) {
		t.Fatalf("want errMissingFlag, got %v", err)
	}
}

func TestValidateActivateResourceArgs_HappyPath(t *testing.T) {
	err := validateActivateResourceArgs(activateResourceArgs{
		Role: "Contributor", Scope: "/subscriptions/x",
		Ticket: "Jira:1", Justification: "j", Duration: 60,
	}, true)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
