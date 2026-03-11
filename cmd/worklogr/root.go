package main

import (
	"strings"

	"github.com/spf13/cobra"
)

type rootOptions struct {
	configPath string
}

var rootCmd = newRootCmd()

func newRootCmd() *cobra.Command {
	options := &rootOptions{}
	cmd := &cobra.Command{
		Use:   "worklogr",
		Short: "報告書生成のためのマルチサービスイベント収集CLI",
		Long: `worklogrは複数のサービス（Slack、GitHub、Google Calendar）からイベントを収集し、
SQLiteに保存して、JSON/CSV（AI向けJSONを含む）で出力するCLIツールです。

Google Calendarはgcloud認証（ADC）を使用し、イベントに添付されたGoogleドキュメント（Geminiメモ等）の本文テキストも収集できます。
添付本文はイベント本体とは別テーブル（event_attachments）に保存され、AI向けJSONでは context.attachments に含まれます。`,
	}

	cmd.PersistentFlags().StringVarP(&options.configPath, "config", "c", "", "設定ファイルのパス")
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetUsageTemplate(`使用方法:
  {{.UseLine}}{{if .HasAvailableSubCommands}}

利用可能なコマンド:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

フラグ:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

グローバルフラグ:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

追加のヘルプトピック:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

詳細については "{{.CommandPath}} [command] --help" を使用してください。{{end}}
`)

	helpCmd := &cobra.Command{
		Use:   "help [command]",
		Short: "任意のコマンドのヘルプ",
		Long: `任意のコマンドのヘルプ情報を表示します。
引数なしで実行すると、利用可能なすべてのコマンドを表示します。`,
		Run: func(c *cobra.Command, args []string) {
			cmd, _, err := c.Root().Find(args)
			if cmd == nil || err != nil {
				c.Printf("不明なコマンド \"%s\" です。\n", strings.Join(args, " "))
				c.Root().Usage()
				return
			}

			cmd.InitDefaultHelpFlag()
			cmd.Help()
		},
	}
	cmd.SetHelpCommand(helpCmd)
	cmd.AddCommand(newGCloudCmd())
	cmd.AddCommand(newCollectCmd(options))
	cmd.AddCommand(newExportCmd(options))
	cmd.AddCommand(newStatusCmd(options))
	cmd.AddCommand(newConfigCmd(options))

	return cmd
}
