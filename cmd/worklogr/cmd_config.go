package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newConfigCmd(rootOptions *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "設定を管理",
		Long:  "worklogrの設定を表示・管理します",
	}
	cmd.AddCommand(newConfigShowCmd(rootOptions))
	return cmd
}

func newConfigShowCmd(rootOptions *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "現在の設定を表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCLIConfig(rootOptions.configPath)
			if err != nil {
				return err
			}

			fmt.Println("現在の設定:")
			fmt.Println("===========")
			fmt.Printf("データベースパス: %s\n", cfg.DatabasePath)
			fmt.Printf("タイムゾーン: %s\n", cfg.Timezone)
			fmt.Println("\nサービス:")

			services := map[string]struct {
				displayName string
				configured  bool
				enabled     bool
			}{
				"slack": {
					displayName: serviceDisplayNames["slack"],
					configured:  cfg.Slack.ClientID != "" && cfg.Slack.ClientSecret != "",
					enabled:     cfg.Slack.Enabled,
				},
				"github": {
					displayName: serviceDisplayNames["github"],
					configured:  cfg.GitHub.ClientID != "" && cfg.GitHub.ClientSecret != "",
					enabled:     cfg.GitHub.Enabled,
				},
				"google_calendar": {
					displayName: serviceDisplayNames["google_calendar"],
					configured:  cfg.GoogleCal.ClientID != "" && cfg.GoogleCal.ClientSecret != "",
					enabled:     cfg.GoogleCal.Enabled,
				},
			}

			for _, serviceName := range orderedServiceNames {
				service := services[serviceName]
				fmt.Printf("  %-15s: 有効=%t, 設定済み=%t\n",
					service.displayName,
					service.enabled,
					service.configured)
			}

			return nil
		},
	}
}
