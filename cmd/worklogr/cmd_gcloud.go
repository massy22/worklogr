package main

import (
	"fmt"

	"github.com/iriam/worklogr/internal/auth"
	"github.com/spf13/cobra"
)

func newGCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gcloud",
		Short: "Google（Calendar/Drive）のgcloud認証（ADC）を確認/案内",
		Long:  "Google Calendar（およびGeminiメモ等のGoogleドキュメント添付取得）に必要なgcloud認証（Application Default Credentials）を確認/案内します。",
	}

	cmd.AddCommand(newGCloudStatusCmd())
	cmd.AddCommand(newGCloudSetupCmd())
	return cmd
}

func newGCloudStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "gcloud認証状態を確認",
		RunE: func(cmd *cobra.Command, args []string) error {
			calendarAuth := auth.NewCalendarAuthManagerWithGCloud(&auth.AuthConfig{}, nil)

			if calendarAuth.IsGCloudAuthenticated() {
				fmt.Println("gcloud認証が利用可能です")
				fmt.Println("Google Calendar/Driveにgcloud認証でアクセスできます")
			} else {
				fmt.Println("gcloud認証が利用できません")
				fmt.Println("\ngcloud認証をセットアップするには:")
				fmt.Println(calendarAuth.SetupGCloudAuth())
			}

			return nil
		},
	}
}

func newGCloudSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "gcloudセットアップ手順を表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			calendarAuth := auth.NewCalendarAuthManagerWithGCloud(&auth.AuthConfig{}, nil)
			fmt.Println(calendarAuth.SetupGCloudAuth())
			return nil
		},
	}
}
