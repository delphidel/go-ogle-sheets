/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go-ogle-sheets/api"
	"go-ogle-sheets/conf"
	"log"
	"strings"
)

var cleanConfig conf.CleanConfig

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove some set of generated turnout sheets",
	Long:  `Remove some set of generated turnout sheets`,
	Run: func(cmd *cobra.Command, args []string) {
		var driveFiles []*api.DriveFile
		var err error
		if cleanConfig.Q != "" {
			driveFiles, err = api.AllSpreadsheetsByQ(cleanConfig.Q)
		} else if cleanConfig.MatchPattern != "" {
			driveFiles, err = api.AllSpreadsheetsByPartialName(cleanConfig.MatchPattern)
		} else {
			driveFiles, err = api.AllSpreadsheetsByPartialName(api.SpreadsheetNamePrefixFromDate(cleanConfig.Date))
		}
		if err != nil {
			log.Fatalf("Failed to get spreadsheets by name: %v", err)
		}

		names := make([]string, len(driveFiles))
		ids := make([]string, len(driveFiles))
		for i, f := range driveFiles {
			names[i] = f.Name
			ids[i] = f.Id
		}

		if len(driveFiles) == 0 {
			fmt.Println("Found no matches.")
		} else {
			fmt.Printf("Found %d matches. IDs: %s\n", len(driveFiles), strings.Join(names, ", "))
			if !cleanConfig.Test {
				// Confirmation might as well live here, def not in the api client wrapper layer...
				var confirm string
				fmt.Printf("Delete %d spreadsheets? (only 'yes' will be accepted): ", len(driveFiles))
				fmt.Scan(&confirm)
				// TODO: parallelize w/ pure channels for practice
				if confirm == "yes" {
					// TODO: concurrency in this loop
					for _, id := range ids {
						err := api.DeleteSpreadsheet(id)
						if err != nil {
							log.Fatalf("Failed to delete spreadsheet %s: %v", id, err)
						}
					}
					fmt.Printf("Deleted %d spreadsheets\n", len(driveFiles))
				} else {
					fmt.Println("Not deleting.")
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().StringVarP(&cleanConfig.Date, "date", "d", "", "Date for created spreadsheet titles")
	cleanCmd.Flags().StringVarP(&cleanConfig.MatchPattern, "match", "m", "", "Pattern to match (will override --date specification)")
	cleanCmd.Flags().StringVarP(&cleanConfig.Q, "q", "q", "", "Full Google API query")
	cleanCmd.MarkFlagsMutuallyExclusive("date", "match", "q")
	cleanCmd.MarkFlagsOneRequired("date", "match", "q")

	cleanCmd.Flags().BoolVarP(&cleanConfig.Test, "test", "t", false, "If passed, only print matching files and do not delete")

}
