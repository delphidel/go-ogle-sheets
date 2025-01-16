package api

import (
	"context"
	"log"
	"os"

	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"google.golang.org/api/drive/v3"
	"go-ogle-sheets/conf"
	"net/http"
)

var sheetsService *sheets.Service
var driveService *drive.Service

func GenerateAllBatches(config conf.GenerationConfig) error {
	names, numbers, err := getNamesAndNumbers(config.TurnoutSourceId, config.TurnoutReadRange, config.DoTurnoutIdx, config.FirstNameIdx, config.PhoneIdx)
	if err != nil {
		log.Printf("Error in GetNamesAndNumbers: %v", err)
		return err
	}
	batches := calculateBatches(len(names), config.BatchSize, config.LastPageFudgeFactor)
	titles := make([]string, batches)
	log.Printf("Generating and filling %d spreadsheets", batches)
	for i := range batches {
		titles[i] = SpreadsheetNameFromDate(config.Date, i+1)
		spreadsheet, err := CreateEmptySpreadsheet(titles[i])
		if err != nil {
			log.Printf("Error in CreateEmptySpreadsheet: %v", err)
			return err
		}

		_, err = copyTemplateIntoSheet(config.TurnoutSourceId, config.TemplateSheetId, spreadsheet.SpreadsheetId)
		if err != nil {
			log.Printf("Error in CopyTemplateIntoSheet: %v", err)
			return err
		}

		_, err = insertBatchIntoSheet(names, numbers, spreadsheet.SpreadsheetId, i, config.BatchSize, i >= batches-1, config.LastPageFudgeFactor)
		if err != nil {
			log.Printf("Error in InsertBatchIntoSheet: %v", err)
			return err
		}
	}
	log.Printf("Successfully generated %d spreadsheets!", batches)
	for i := range(titles) {
		log.Print(titles[i])
	}
	return nil
}

// This is goofy, but I'm just cruising through how go works again
type DriveFile struct {
	Name string
	Id string
}

func AllSpreadsheetsByNamePrefix(namePart string) ([]*DriveFile, error) {
	fileList, err := driveService.Files.List().Q(fmt.Sprintf("mimeType = 'application/vnd.google-apps.spreadsheet' and name contains '%s'", namePart)).Do()
	if err != nil {
		log.Printf("Error finding spreadsheets by name: %v", err)
		return nil, err
	}
	driveFiles := make([]*DriveFile, len(fileList.Files))
	for i, f := range fileList.Files {
		driveFiles[i] = &DriveFile{Name: f.Name, Id: f.Id}
	}
	return driveFiles, nil
}

func DeleteSpreadsheet(spreadsheetId string) error {
	return driveService.Files.Delete(spreadsheetId).Do()
}

func SpreadsheetNameFromDate(date string, group int) string {
	return fmt.Sprintf("%s - Group %d", SpreadsheetNamePrefixFromDate(date), group)
}

// TODO: Configurable base name
func SpreadsheetNamePrefixFromDate(date string) string {
	return fmt.Sprintf("IC Turnout - %s", date)
}

func insertBatchIntoSheet(names []interface{}, numbers []interface{}, targetSpreadsheetId string, batchIdx int, batchSize int, isLastBatch bool, lastPageFudgeFactor int) (*sheets.UpdateValuesResponse, error) {
	// Calculate target range
	n := len(names)
	offset := (batchIdx) * batchSize // 0, 10, 20, ...
	batchRows := batchSize
	if isLastBatch { // last batch is special
		if n%batchSize <= lastPageFudgeFactor { // throw the last few in the same batch
			batchRows = batchSize + n%batchSize
		} else { // last batch is just the remainder
			batchRows = n % batchSize
		}
	}

	// TODO: randomize order

	// Create insertValues as slice of columns
	insertValues := make([][]interface{}, 2)
	insertValues[0] = names[offset : offset+batchRows]
	insertValues[1] = numbers[offset : offset+batchRows]

	// Write names and numbers to new sheet
	// TODO: fix the name of the sheet and then update it here
	log.Printf("Inserting batch of %d into target table", len(insertValues[0]))
	valueRange := "Copy of turnout-template!A2:B"
	return sheetsService.Spreadsheets.Values.Update(targetSpreadsheetId, valueRange, &sheets.ValueRange{
		MajorDimension: "COLUMNS",
		Range:          valueRange,
		Values:         insertValues,
	}).ValueInputOption("RAW").Do()
}

// TODO: Remove "Sheet1", rename "Copy of Sheet1" to "Sheet1" (or equivalent)
func copyTemplateIntoSheet(turnoutSourceId string, templateSheetId int64, targetSpreadsheetId string) (*sheets.SheetProperties, error) {
	// Copy template sheet to new sheet
	log.Print("Copying template into spreadsheet")
	return sheetsService.Spreadsheets.Sheets.CopyTo(turnoutSourceId, templateSheetId, &sheets.CopySheetToAnotherSpreadsheetRequest{
		DestinationSpreadsheetId: targetSpreadsheetId,
	}).Do()
}

func CreateEmptySpreadsheet(title string) (*sheets.Spreadsheet, error) {
	log.Printf("Creating empty spreadsheet %s", title)
	return sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: title,
		},
	}).Do()
}

func calculateBatches(numRows int, batchSize int, lastPageFudgeFactor int) int {
	// Calculate number of batches
	if numRows%batchSize <= lastPageFudgeFactor {
		return numRows / batchSize
	}
	return numRows/batchSize + 1
}

func getNamesAndNumbers(turnoutSourceId string, turnoutReadRange string, doTurnoutIdx int, firstNameIdx int, phoneIdx int) ([]interface{}, []interface{}, error) {
	resp, err := sheetsService.Spreadsheets.Values.Get(turnoutSourceId, turnoutReadRange).Do()
	if err != nil {
		log.Printf("Unable to retrieve data from spreadsheet: %v", err)
		return nil, nil, err
	}
	// Extract names and phone numbers
	// TODO: this number is super wrong
	// TODO: no error case right now
	log.Printf("Got %d rows from source sheet", len(resp.Values))
	names := make([]interface{}, 0, len(resp.Values))
	numbers := make([]interface{}, 0, len(resp.Values))
	for _, row := range resp.Values {
		// Assign columns to variables
		if len(row) == 4 && row[doTurnoutIdx] == "TRUE" {
			names = append(names, row[firstNameIdx])
			numbers = append(numbers, row[phoneIdx])
		}
	}
	return names, numbers, nil
}

func getClient() (*http.Client, error) {
	// Get creds
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, err
	}

	authConfig, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/drive")
	if err != nil {
		return nil, err
	}
	return GetClient(authConfig), nil
}

func getSheetsService(client *http.Client, ctx context.Context) (srv *sheets.Service, err error) {
	srv, err = sheets.NewService(ctx, option.WithHTTPClient(client))
	return
}

func getDriveService(client *http.Client, ctx context.Context) (srv *drive.Service, err error) {
	srv, err = drive.NewService(ctx, option.WithHTTPClient(client))
	return
}

func init() {
	ctx := context.Background()
	client, err := getClient()
	if err != nil {
		log.Fatalf("Failed to create Google API client: %v", err)
	}

	sheetsService, err = getSheetsService(client, ctx)
	if err != nil {
		log.Fatalf("Failed to create Google Sheets API service: %v", err)
	}

	driveService, err = getDriveService(client, ctx)
	if err != nil {
		log.Fatalf("Failed to create Google Drive API service: %v", err)
	}
}
