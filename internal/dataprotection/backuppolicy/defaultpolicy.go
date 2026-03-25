package backuppolicy

import (
  "encoding/json"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dataprotection/armdataprotection/v3"
  "github.com/spf13/cobra"
)

func newGetDefaultPolicyTemplateCommand() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "get-default-policy-template",
    Short: "Get the default policy template for a datasource type",
    Long:  "Returns a default backup policy JSON template for the specified datasource type",
    RunE: func(cmd *cobra.Command, args []string) error {
      datasourceType, _ := cmd.Flags().GetString("datasource-type")
      return GetDefaultPolicyTemplate(datasourceType)
    },
  }
  cmd.Flags().String("datasource-type", "", "Datasource type (e.g., AzureDatabaseForPostgreSQLFlexibleServer)")
  cmd.MarkFlagRequired("datasource-type")
  return cmd
}

func GetDefaultPolicyTemplate(datasourceType string) error {
  template, err := buildDefaultTemplate(datasourceType)
  if err != nil {
    return err
  }

  output, err := json.MarshalIndent(template, "", "  ")
  if err != nil {
    return fmt.Errorf("failed to format policy template: %w", err)
  }

  fmt.Println(string(output))
  return nil
}

func buildDefaultTemplate(datasourceType string) (*armdataprotection.BaseBackupPolicyResource, error) {
  switch datasourceType {
  case "AzureDatabaseForPostgreSQLFlexibleServer":
    return buildPGFlexDefaultPolicy(), nil
  default:
    return nil, fmt.Errorf("unsupported datasource type: %s. Supported types: AzureDatabaseForPostgreSQLFlexibleServer", datasourceType)
  }
}

func buildPGFlexDefaultPolicy() *armdataprotection.BaseBackupPolicyResource {
  backupSchedule := "R/2024-01-01T00:00:00+00:00/P1W" // Weekly
  vaultStore := armdataprotection.DataStoreTypesVaultStore

  return &armdataprotection.BaseBackupPolicyResource{
    Properties: &armdataprotection.BackupPolicy{
      ObjectType:     to.Ptr("BackupPolicy"),
      DatasourceTypes: []*string{to.Ptr("Microsoft.DBforPostgreSQL/flexibleServers")},
      PolicyRules: []armdataprotection.BasePolicyRuleClassification{
        &armdataprotection.AzureBackupRule{
          Name:       to.Ptr("BackupWeekly"),
          ObjectType: to.Ptr("AzureBackupRule"),
          DataStore: &armdataprotection.DataStoreInfoBase{
            DataStoreType: &vaultStore,
            ObjectType:    to.Ptr("DataStoreInfoBase"),
          },
          BackupParameters: &armdataprotection.AzureBackupParams{
            ObjectType: to.Ptr("AzureBackupParams"),
            BackupType: to.Ptr("Full"),
          },
          Trigger: &armdataprotection.ScheduleBasedTriggerContext{
            ObjectType: to.Ptr("ScheduleBasedTriggerContext"),
            Schedule: &armdataprotection.BackupSchedule{
              RepeatingTimeIntervals: []*string{&backupSchedule},
            },
            TaggingCriteria: []*armdataprotection.TaggingCriteria{
              {
                IsDefault:       to.Ptr(true),
                TaggingPriority: to.Ptr[int64](99),
                TagInfo: &armdataprotection.RetentionTag{
                  TagName: to.Ptr("Default"),
                },
              },
            },
          },
        },
        &armdataprotection.AzureRetentionRule{
          Name:       to.Ptr("Default"),
          ObjectType: to.Ptr("AzureRetentionRule"),
          IsDefault:  to.Ptr(true),
          Lifecycles: []*armdataprotection.SourceLifeCycle{
            {
              DeleteAfter: &armdataprotection.AbsoluteDeleteOption{
                ObjectType: to.Ptr("AbsoluteDeleteOption"),
                Duration:   to.Ptr("P3M"),
              },
              SourceDataStore: &armdataprotection.DataStoreInfoBase{
                DataStoreType: &vaultStore,
                ObjectType:    to.Ptr("DataStoreInfoBase"),
              },
            },
          },
        },
      },
    },
  }
}
