package main

import (
	"fmt"

	"github.com/iriam/worklogr/internal/app"
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
	usecase := app.NewConfigShowUsecase()

	return &cobra.Command{
		Use:   "show",
		Short: "現在の設定を表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := usecase.Run(app.ConfigShowRequest{ConfigPath: rootOptions.configPath})
			if err != nil {
				return err
			}

			fmt.Println("現在の設定:")
			fmt.Println("===========")
			fmt.Printf("データベースパス: %s\n", result.DatabasePath)
			fmt.Printf("タイムゾーン: %s\n", result.Timezone)
			fmt.Println("\nサービス:")

			for _, service := range result.Services {
				fmt.Printf("  %-15s: 有効=%t, 設定済み=%t\n",
					service.DisplayName,
					service.Enabled,
					service.Configured)
			}

			return nil
		},
	}
}
