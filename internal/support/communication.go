package support

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
	"github.com/spf13/cobra"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
)

func newCommunicationCmd(tenantLevel bool) *cobra.Command {
	create := &cobra.Command{
		Use:   "create",
		Short: "Add a communication to a support ticket",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			ticket, name := str(cmd, "ticket-name"), str(cmd, "communication-name")
			if err := c.CheckCommunicationName(cmd.Context(), ticket, name); err != nil {
				return err
			}
			res, err := c.CreateCommunication(cmd.Context(), ticket, name, armsupport.CommunicationDetails{
				Properties: &armsupport.CommunicationDetailsProperties{
					Subject: ref(str(cmd, "communication-subject")),
					Body:    ref(str(cmd, "communication-body")),
					Sender:  strp(cmd, "communication-sender"),
				},
			})
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, res)
		},
	}
	addStr(create, "ticket-name", "Support ticket name.", true)
	addStr(create, "communication-name", "Communication name.", true)
	addStr(create, "communication-subject", "Subject of the communication.", true)
	addStr(create, "communication-body", "Body of the communication.", true)
	addStr(create, "communication-sender", "Email address of the sender.", false)

	list := &cobra.Command{
		Use:   "list",
		Short: "List the communications of a support ticket",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			items, err := c.ListCommunications(cmd.Context(), str(cmd, "ticket-name"), str(cmd, "filter"), maxItems(cmd))
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, listOrEmpty(items))
		},
	}
	addStr(list, "ticket-name", "Support ticket name.", true)
	addStr(list, "filter", "OData filter, for example \"CreatedDate ge 2026-11-01\".", false)
	addListFlags(list)

	show := &cobra.Command{
		Use:   "show",
		Short: "Show a communication of a support ticket",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			res, err := c.GetCommunication(cmd.Context(), str(cmd, "ticket-name"), str(cmd, "communication-name"))
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, res)
		},
	}
	addStr(show, "ticket-name", "Support ticket name.", true)
	addStr(show, "communication-name", "Communication name.", true)

	return group("communication", "Manage ticket communications", create, list, show)
}

func newChatTranscriptCmd(tenantLevel bool) *cobra.Command {
	list := &cobra.Command{
		Use:   "list",
		Short: "List the chat transcripts of a support ticket",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			items, err := c.ListChatTranscripts(cmd.Context(), str(cmd, "ticket-name"), maxItems(cmd))
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, listOrEmpty(items))
		},
	}
	addStr(list, "ticket-name", "Support ticket name.", true)
	addListFlags(list)

	show := &cobra.Command{
		Use:   "show",
		Short: "Show a chat transcript of a support ticket",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			res, err := c.GetChatTranscript(cmd.Context(), str(cmd, "ticket-name"), str(cmd, "chat-transcript-name"))
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, res)
		},
	}
	addStr(show, "ticket-name", "Support ticket name.", true)
	addStr(show, "chat-transcript-name", "Chat transcript name.", true)

	return group("chat-transcript", "View ticket chat transcripts", list, show)
}
