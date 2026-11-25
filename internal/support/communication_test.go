package support

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommunicationCreateChecksName(t *testing.T) {
	f := newFake()
	_, err := runCmd(t, f, "in-subscription", "communication", "create", "--ticket-name", "t1", "--communication-name", "c1",
		"--communication-subject", "S", "--communication-body", "B", "--communication-sender", "a@x")
	require.NoError(t, err)
	require.Equal(t, []string{"CheckCommunicationName:t1/c1", "CreateCommunication:t1/c1"}, f.calls)
	p := f.comms["t1"][0].Properties
	require.Equal(t, "S", *p.Subject)
	require.Equal(t, "B", *p.Body)
	require.Equal(t, "a@x", *p.Sender)
}

func TestCommunicationCreateNameTaken(t *testing.T) {
	f := newFake()
	f.takenNames["c1"] = true
	_, err := runCmd(t, f, "in-subscription", "communication", "create", "--ticket-name", "t1", "--communication-name", "c1",
		"--communication-subject", "S", "--communication-body", "B")
	require.Error(t, err)
	require.Equal(t, []string{"CheckCommunicationName:t1/c1"}, f.calls)
}
