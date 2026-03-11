package main

import (
	"fmt"

	"github.com/iriam/worklogr/internal/app"
	"github.com/spf13/cobra"
)

func newStatusCmd(rootOptions *rootOptions) *cobra.Command {
	usecase := app.NewStatusUsecase()

	return &cobra.Command{
		Use:   "status",
		Short: "サービス状態を表示",
		Long:  "設定されたすべてのサービスの状態を表示します",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := usecase.Run(app.StatusRequest{ConfigPath: rootOptions.configPath})
			if err != nil {
				return err
			}

			fmt.Println("サービス状態:")
			fmt.Println("=============")

			for _, serviceName := range orderedServiceNames {
				if serviceStatus, exists := result.ServiceStatus[serviceName]; exists {
					fmt.Printf("%-15s | 有効: %-5t | 認証済み: %-5t | 初期化済み: %-5t\n",
						serviceDisplayNames[serviceName],
						serviceStatus.Enabled,
						serviceStatus.Authenticated,
						serviceStatus.Initialized)
				}
			}

			fmt.Println("\nデータベース統計:")
			fmt.Println("=================")
			if result.StatsError != nil {
				fmt.Printf("%v\n", result.StatsError)
			} else {
				for service, count := range result.Stats {
					fmt.Printf("%-15s: %d イベント\n", serviceDisplayNames[service], count)
				}
			}

			return nil
		},
	}
}
