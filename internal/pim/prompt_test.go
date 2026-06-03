package pim

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPromptStringNonInteractive(t *testing.T) {
	p := &Prompter{In: strings.NewReader(""), Out: &bytes.Buffer{}, IsTTY: false}
	_, err := p.PromptString("name")
	if !errors.Is(err, ErrNonInteractive) {
		t.Fatalf("want ErrNonInteractive, got %v", err)
	}
}

func TestPromptStringInteractiveReadsLine(t *testing.T) {
	p := &Prompter{In: strings.NewReader("acme-prod\n"), Out: &bytes.Buffer{}, IsTTY: true}
	got, err := p.PromptString("scope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "acme-prod" {
		t.Fatalf("got %q want %q", got, "acme-prod")
	}
}

func TestPickAssignmentNonInteractive(t *testing.T) {
	p := &Prompter{In: strings.NewReader(""), Out: &bytes.Buffer{}, IsTTY: false}
	_, err := p.PickAssignment("choose", []string{"a", "b"})
	if !errors.Is(err, ErrNonInteractive) {
		t.Fatalf("want ErrNonInteractive, got %v", err)
	}
}

func TestPickAssignmentSelectsByNumber(t *testing.T) {
	p := &Prompter{In: strings.NewReader("2\n"), Out: &bytes.Buffer{}, IsTTY: true}
	idx, err := p.PickAssignment("choose", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 {
		t.Fatalf("got idx %d want 1", idx)
	}
}

func TestPickAssignmentRejectsOutOfRange(t *testing.T) {
	p := &Prompter{In: strings.NewReader("9\n"), Out: &bytes.Buffer{}, IsTTY: true}
	_, err := p.PickAssignment("choose", []string{"a", "b"})
	if err == nil {
		t.Fatal("want error for out-of-range index")
	}
}
