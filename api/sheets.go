package api

import (
	"context"
	"log"
	"os"

	"errors"
	"fmt"
	"go-ogle-sheets/conf"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"net/http"
	"go-ogle-sheets/util"
)

var sheetsService *sheets.Service
var driveService *drive.Service

func GenerateAllBatches(config conf.GenerationConfig) error {
	log.Printf("Gathering source data...")
	names, numbers, err := getNamesAndNumbers(config.TurnoutSourceId, config.TurnoutReadRange, config.DoTurnoutIdx, config.FirstNameIdx, config.PhoneIdx)
	if err != nil {
		log.Printf("Error in GetNamesAndNumbers: %v", err)
		return err
	}
	// Randomize names & numbers
	randomized := util.ShuffleSlices([][]interface{}{names, numbers})
	names = randomized[0]
	numbers = randomized[1]

	batches := calculateBatches(len(names), config.BatchSize, config.LastPageFudgeFactor)
	titles := make([]string, batches)
	log.Printf("Generating and filling %d spreadsheets", batches)

	// Concurrently create each batch
	ch := make(chan error, config.Concurrency)
	for i := range batches {
		go func(ch chan error) {
			titles[i] = SpreadsheetNameFromDate(config.Date, i+1)
			spreadsheet, err := CreateEmptySpreadsheet(titles[i])
			if err != nil {
				log.Printf("Error in CreateEmptySpreadsheet: %v", err)
			}

			err = copyTemplateIntoSheet(config.TurnoutSourceId, config.TemplateSheetId, spreadsheet)
			if err != nil {
				log.Printf("Error in CopyTemplateIntoSheet: %v", err)
			}

			_, err = insertBatchIntoSheet(names, numbers, spreadsheet.SpreadsheetId, i, config.BatchSize, i >= batches-1, config.LastPageFudgeFactor)
			if err != nil {
				log.Printf("Error in InsertBatchIntoSheet: %v", err)
			}
			ch<-err
		}(ch)
	}
	errs := make([]error, 0, batches)
	for i := 0; i < batches; i++ {
		e := <-ch
		if e != nil {
			errs = append(errs, e)
		}
		if i >= batches { // TODO: this is clumsy
			close(ch)
		}
	}
	if len(errs) > 0 {
		log.Printf("Errors while generating spreadsheets: %v", errors.Join(errs...))
	}
	fmt.Printf("Successfully generated %d spreadsheets!\n", batches)
	for i := range titles {
		fmt.Println(titles[i])
	}
	return errors.Join(errs...)
}

// This is goofy, but I'm just cruising through how go works again
type DriveFile struct {
	Name string
	Id   string
}

func AllSpreadsheetsByPartialName(namePart string) ([]*DriveFile, error) {
	return AllSpreadsheetsByQ(fmt.Sprintf("name contains '%s'", namePart))
}

func AllSpreadsheetsByQ(q string) ([]*DriveFile, error) {
	endQ := fmt.Sprintf("mimeType = 'application/vnd.google-apps.spreadsheet' and %s", q)
	fileList, err := driveService.Files.List().Q(endQ).Do()
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

	// Create insertValues as slice of columns
	insertValues := make([][]interface{}, 2)
	insertValues[0] = names[offset : offset+batchRows]
	insertValues[1] = numbers[offset : offset+batchRows]

	// Write names and numbers to new sheet
	log.Printf("Inserting batch of %d into target table", len(insertValues[0]))
	valueRange := "Sheet1!A2:B"
	return sheetsService.Spreadsheets.Values.Update(targetSpreadsheetId, valueRange, &sheets.ValueRange{
		MajorDimension: "COLUMNS",
		Range:          valueRange,
		Values:         insertValues,
	}).ValueInputOption("RAW").Do()
}

func copyTemplateIntoSheet(turnoutSourceId string, templateSheetId int64, targetSpreadsheet *sheets.Spreadsheet) error {
	// Copy template sheet to new sheet
	log.Print("Copying template into new spreadsheet")
	newSheetProperties, err := sheetsService.Spreadsheets.Sheets.CopyTo(turnoutSourceId, templateSheetId, &sheets.CopySheetToAnotherSpreadsheetRequest{
		DestinationSpreadsheetId: targetSpreadsheet.SpreadsheetId,
	}).Do()
	if err != nil {
		log.Printf("Error copying template into spreadsheet: %v", err)
		return err
	}

	// Note: this object is from the past, so it will not reflect the above update
	// TODO: maybe check this error when it's actually created to avoid this paradox?
	if len(targetSpreadsheet.Sheets) != 1 {
		return errors.New(fmt.Sprintf("Brand-new spreadsheet %s should have only one sheet! Got %d", targetSpreadsheet.Properties.Title, len(targetSpreadsheet.Sheets)))
	}
	// Delete default empty sheet
	log.Printf("Removing empty sheet from new spreadsheet")
	_, err = sheetsService.Spreadsheets.BatchUpdate(targetSpreadsheet.SpreadsheetId, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				DeleteSheet: &sheets.DeleteSheetRequest{SheetId: targetSpreadsheet.Sheets[0].Properties.SheetId},
			},
		},
	}).Do()
	if err != nil {
		log.Printf("Error removing empty sheet from spreadsheet %s: %v", targetSpreadsheet.Properties.Title, err)
		return err
	}

	// Rename copied sheet to 'Sheet1'
	log.Printf("Renaming new sheet to 'Sheet1'")
	_, err = sheetsService.Spreadsheets.BatchUpdate(targetSpreadsheet.SpreadsheetId, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
					Fields: fmt.Sprintf("title"),
					Properties: &sheets.SheetProperties{
						SheetId: newSheetProperties.SheetId,
						Title: "Sheet1",
					},
				},
			},
		},
	}).Do()
	return nil
}

// TODO: it would be fun if this were idempotent by title
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
