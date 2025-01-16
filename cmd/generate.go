/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
	"go-ogle-sheets/api"
	"go-ogle-sheets/conf"
	"log"
	"strconv"
)

var genConfig conf.GenerationConfig

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Main function: generate turnout sheets",
	Long:  `Generate turnout sheets 10 at a time based on the source sheet`,
	Run: func(cmd *cobra.Command, args []string) {
		err := api.GenerateAllBatches(genConfig)
		if err != nil {
			log.Fatalf("Failed to create spreadsheets: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	// Main flags (should not ship with default ids embedded obv)
	generateCmd.Flags().StringVarP(&genConfig.Date, "date", "d", "", "Date for created spreadsheet titles")
	generateCmd.MarkFlagRequired("date")

	generateCmd.Flags().StringVarP(&genConfig.TurnoutSourceId, "source", "s", "15bc-ViIr9Q1tP3xKpl79wGyVmr1UsBlQSrv_GkVVEzA", "ID of source spreadsheet")

	// Need int64 for this flag
	var templateSheetIdStr string
	generateCmd.Flags().StringVarP(&templateSheetIdStr, "template-sheet", "t", "1625421409", "ID of template sheet in source spreadsheet")
	templateSheetId, err := strconv.ParseInt(templateSheetIdStr, 10, 64)
	if err != nil {
		log.Fatalf("Could not convert %s to int64: %v", templateSheetIdStr, err)
	}
	genConfig.TemplateSheetId = templateSheetId

	generateCmd.Flags().IntVar(&genConfig.DoTurnoutIdx, "do-turnout-idx", 0, "Relative Index of Do Turnout field (default 0)")
	generateCmd.Flags().IntVar(&genConfig.FirstNameIdx, "first-name-idx", 1, "Relative Index of First Name field (default 1)")
	generateCmd.Flags().IntVar(&genConfig.PhoneIdx, "phone-idx", 3, "Relative Index of Phone Number field (default 3)")
	generateCmd.Flags().IntVar(&genConfig.BatchSize, "batch-size", 10, "Number of records per batch (default 10)")
	generateCmd.Flags().IntVar(&genConfig.LastPageFudgeFactor, "last-page-fudge", 3, "Maximum number of records to append to last batch (default 3)")
	generateCmd.Flags().StringVarP(&genConfig.TurnoutReadRange, "read-range", "r", "turnout-list!B2:E", "A1-style read range to pull from source spreadsheet")
}
