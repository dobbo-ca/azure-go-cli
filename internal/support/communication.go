package support

import "github.com/spf13/cobra"

func newCommunicationCmd(tenantLevel bool) *cobra.Command {
	return group("communication", "Manage ticket communications")
}
func newChatTranscriptCmd(tenantLevel bool) *cobra.Command {
	return group("chat-transcript", "View ticket chat transcripts")
}
