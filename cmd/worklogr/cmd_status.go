package main

import (
	"fmt"

	"github.com/iriam/worklogr/internal/collector"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "サービス状態を表示",
	Long:  "設定されたすべてのサービスの状態を表示します",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, db, err := loadCLIConfigAndDatabase()
		if err != nil {
			return err
		}
		defer db.Close()

		eventCollector := collector.NewEventCollector(cfg, db)
		eventCollector.InitializeServices()
		status := eventCollector.GetServiceStatus()

		fmt.Println("サービス状態:")
		fmt.Println("=============")

		for _, serviceName := range orderedServiceNames {
			if serviceStatus, exists := status[serviceName]; exists {
				fmt.Printf("%-15s | 有効: %-5t | 認証済み: %-5t | 初期化済み: %-5t\n",
					serviceDisplayNames[serviceName],
					serviceStatus.Enabled,
					serviceStatus.Authenticated,
					serviceStatus.Initialized)
			}
		}

		fmt.Println("\nデータベース統計:")
		fmt.Println("=================")
		stats, err := db.GetStats()
		if err != nil {
			fmt.Printf("データベース統計の取得に失敗しました: %v\n", err)
		} else {
			for service, count := range stats {
				fmt.Printf("%-15s: %d イベント\n", serviceDisplayNames[service], count)
			}
		}

		return nil
	},
}
