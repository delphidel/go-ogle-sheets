package conf

type GenerationConfig struct {
	Date string
	TurnoutSourceId string
	TurnoutReadRange string
	TemplateSheetId int64
	DoTurnoutIdx int
	FirstNameIdx int
	PhoneIdx int
	BatchSize int
	LastPageFudgeFactor int
}

type CleanConfig struct {
	Date string
	Test bool
}
