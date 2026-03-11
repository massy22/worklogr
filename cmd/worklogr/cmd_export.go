package main

import (
	"fmt"

	"github.com/iriam/worklogr/internal/app"
	"github.com/spf13/cobra"
)

type exportOptions struct {
	startDate  string
	endDate    string
	services   []string
	outputPath string
	format     string
}

func newExportCmd(rootOptions *rootOptions) *cobra.Command {
	options := &exportOptions{}
	usecase := app.NewExportUsecase()
	cmd := &cobra.Command{
		Use:   "export",
		Short: "収集したイベントをエクスポート",
		Long: `SQLiteからイベントを取得して、指定形式でエクスポートします。

json-ai 形式では、イベントのmetadataに加えて、添付本文（Googleドキュメント等）がある場合は context.attachments に含めます。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			startTime, endTime, err := parseAdjustedTimeRange(options.startDate, options.endDate, rootOptions.configPath)
			if err != nil {
				return fmt.Errorf("時間範囲が無効です: %w", err)
			}

			fmt.Printf("%s から %s までのイベントをエクスポート中...\n",
				startTime.Format("2006-01-02 15:04:05"),
				endTime.Format("2006-01-02 15:04:05"))

			result, err := usecase.Run(app.ExportRequest{
				ConfigPath: rootOptions.configPath,
				StartTime:  startTime,
				EndTime:    endTime,
				Services:   options.services,
				Format:     options.format,
				OutputPath: options.outputPath,
			})
			if err != nil {
				return err
			}

			if result.EventCount == 0 {
				fmt.Println("指定された期間にイベントが見つかりませんでした")
				return nil
			}

			fmt.Printf("エクスポート対象のイベントが %d 件見つかりました\n", result.EventCount)
			return nil
		},
	}

	cmd.Flags().StringVarP(&options.startDate, "start", "s", "", "開始日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringVarP(&options.endDate, "end", "e", "", "終了日時 (YYYY-MM-DD または YYYY-MM-DD HH:MM:SS)")
	cmd.Flags().StringSliceVar(&options.services, "services", []string{}, "エクスポート対象サービス（例: slack,github,google_calendar）")
	cmd.Flags().StringVarP(&options.outputPath, "output", "o", "", "出力ファイルパス")
	cmd.Flags().StringVarP(&options.format, "format", "f", "json", "エクスポート形式 (json, json-ai, csv, csv-summary)")
	cmd.MarkFlagRequired("start")
	cmd.MarkFlagRequired("end")

	return cmd
}
