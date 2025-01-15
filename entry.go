package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// TODO: Break into functions
func main() {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/drive")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	// Request config
	meetingDate := "01/21/2025"
	turnoutMasterId := "15bc-ViIr9Q1tP3xKpl79wGyVmr1UsBlQSrv_GkVVEzA"
	var templateSheetId int64 = 1625421409
	turnoutReadRange := "turnout-master-list!B2:E"
	doTurnoutIdx := 0
	firstNameIdx := 1
	// Throw away last names from col D @ idx 1
	phoneIdx := 3

	// Get values from source spreadsheet
	resp, err := srv.Spreadsheets.Values.Get(turnoutMasterId, turnoutReadRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from spreadsheet: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
	} else {
		// Extract names and phone numbers
		// TODO: this number is super wrong
		log.Printf("Got %d rows from master sheet...", len(resp.Values))
		names := make([]interface{}, 0, len(resp.Values))
		numbers := make([]interface{}, 0, len(resp.Values))
		for _, row := range resp.Values {
			// Assign columns to variables
			if len(row) == 4 && row[doTurnoutIdx] == "TRUE" {
				names = append(names, row[firstNameIdx])
				numbers = append(numbers, row[phoneIdx])
			}
		}

		// Calculate spreadsheet count
		n := len(names)
		batchSize := 10
		lastPageFudgeFactor := 3 // If this many rows are left, stick them on a final page
		var batches int
		if n%batchSize <= lastPageFudgeFactor {
			batches = n / batchSize
		} else {
			batches = n/batchSize + 1
		}

		log.Printf("n=%d. Creating %d spreadsheets of size %d (except for the last one)...", n, batches, batchSize)

		for i := range batches {
			// Create empty spreadsheet

			log.Printf("Creating %dth empty spreadsheet...", i+1)
			spreadsheet, err := srv.Spreadsheets.Create(&sheets.Spreadsheet{
				Properties: &sheets.SpreadsheetProperties{
					Title: fmt.Sprintf("IC Turnout - %s - Group %d", meetingDate, i+1),
				},
			}).Do()
			if err != nil {
				log.Fatalf("Failed to create blank target spreadsheet: %s", err)
			}

			// Copy template sheet to new sheet
			log.Printf("Copying template into %dth spreadsheet...", i+1)
			targetId := spreadsheet.SpreadsheetId
			_, err = srv.Spreadsheets.Sheets.CopyTo(turnoutMasterId, templateSheetId, &sheets.CopySheetToAnotherSpreadsheetRequest{
				DestinationSpreadsheetId: targetId,
			}).Do()
			if err != nil {
				log.Fatalf("Failed to copy template to target spreadsheet: %s", err)
			}

			// Remove "Sheet1", rename "Copy of Sheet1" to "Sheet1" (or equivalent)

			// Calculate target range
			offset := (i) * batchSize // 0, 10, 20, ...
			batchRows := batchSize
			var valueRange string
			if i == batches-1 { // last batch is special
				if n%batchSize <= lastPageFudgeFactor { // throw the last few in the same batch
					batchRows = batchSize + n%batchSize
				} else { // last batch is just the remainder
					batchRows = n % batchSize
				}
			}

			// Create insertValues as slice of columns
			insertValues := make([][]interface{}, 2)
			insertValues[0] = names[offset : offset+batchRows]

			// Write names and numbers to new sheet
			log.Printf("Writing data to new spreadsheet")
			// TODO: fix the name of the sheet and then update it here
			_, err = srv.Spreadsheets.Values.Update(targetId, fmt.Sprintf("Copy of Sheet1!A%d:B", 2), &sheets.ValueRange{
				MajorDimension: "COLUMNS",
				Range:          "Copy of Sheet1!A2:B",
				Values:         insertValues,
			}).ValueInputOption("RAW").Do()
			if err != nil {
				log.Fatalf("Failed to insert values into target spreadsheet at %s: %s", valueRange, err)
			}
		}
	}
	// TODO: Return / output all of the new spreadsheet IDs!
	// TODO: implement a delete behavior w/ CLI switch to remove partial runs etc.
}
