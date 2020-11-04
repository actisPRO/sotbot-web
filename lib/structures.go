package lib

import (
	"time"
)

type LoginData struct {
	Status bool
	OAuth  string
}

type ErrorData struct {
	Message string
}

type RedirectData struct {
	RedirectURL string
}

type IndexData struct {
	Username    string
	AccessLevel int
	Xbox        string
}

type BotCPData struct {
	Status		string
}

type BlacklistEntry struct {
	Id					string
	DiscordId			string
	DiscordUsername		string
	Xbox				string
	Date				time.Time
	Moderator			string
	Reason				string
	Additional			string
}

type BlacklistData struct {
	Entries		[]BlacklistEntry
}