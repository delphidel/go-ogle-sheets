## go-ogle sheets

#### Purpose
- Automate the creation of turnout texting spreadsheets for organizing events 
- Spend a dozen hours to save <10 minutes a week
- Currently, requires a fairly specific format of source spreadsheet, but it will be more extensible in the future

#### Build
- I don't actually know how `go mod` works, it's new since I learned Go 10 years ago
- I think you can just run `go get -u` or something? `go mod tidy`? Then probably `go build` or `go install` maybe.
- It seems like they've landed in the "there are 12 ways to do it but 3 are correct" space they were very carefully avoiding in the early years
- But yeah I fully haven't built this yet, I've just been doing `go run main.go` for testing 

#### Usage
- It's a pretty standard CLI app, `--help` works and some of the messages are informative
- There are two main commands, `generate` and `clean`. They do opposite things, and if you use the `-d` option for date-based naming, they should basically reverse one another.
- The defaults are hardcoded to internal documents, which is definitely bad opsec, but you should be able to override all of them for use in your own system
- I'd like to add features that don't require you to copy Spreadsheet IDs out of the Google URLs

#### Google API Setup
- Google's OAuth implementation is actually terrible
- It made me so sad to set this up
- Deep in the Google API configuration, you can set up a splash screen and scopes for your API client.
- Then, the first time you run this, it'll point you to that splash screen, whence you'll be redirected to localhost, which is 🔥sick🔥.
- This is the behavior desired by the authors of the Google tutorial for the Google API client written in the Google language.
- I'm gonna fix the part that involves extra copy-pasting, but I have not yet found an authentication flow that doesn't involve getting redirected to a localhost url that has your OAuth token hidden in it
