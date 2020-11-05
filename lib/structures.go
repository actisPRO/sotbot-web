package lib

import (
	"database/sql"
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
	Blacklisted bool
}

type BotCPData struct {
	Status		string
}

type BlacklistEntry struct {
	Id					string
	DiscordId			string
	DiscordUsername		sql.NullString
	Xbox				sql.NullString
	Date				time.Time
	DateString			string
	Moderator			string
	ModeratorName		string
	Reason				string
	Additional			sql.NullString
	AdditionalName		string
}

type BlacklistData struct {
	Entries		[]BlacklistEntry
}