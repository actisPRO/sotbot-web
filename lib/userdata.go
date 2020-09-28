package lib

import (
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"time"
)

type UserData struct {
	UserID			string
	RegisteredOn	time.Time
	LastLogin		time.Time
	Username		string
	AvatarURL		string
	Xbox			string
	IP				string
}

func GetUserDataFromDiscord(token string) (UserData, error) {
	bearer, err := discordgo.New("Bearer " + token)
	if err != nil {
		return UserData{}, err
	}

	user, err := bearer.User("@me")
	if err != nil {
		return UserData{}, err
	}

	connections, err := bearer.UserConnections()
	if err != nil {
		return UserData{}, err
	}

	xboxConnection := ""
	for i := 0; i < len(connections); i++ {
		if connections[i].Type == "xbox" {
			xboxConnection = connections[i].Name
		}
	}
	
	data := UserData{
		UserID:    user.ID,
		Username:  user.String(),
		AvatarURL: user.AvatarURL(""),
		Xbox:      xboxConnection,
	}

	_ = bearer.Close()

	return data, nil
}

func GetUserDataFromDB(db *sql.DB, userid string) (UserData, error) {
	data := UserData{}
	err := db.QueryRow(fmt.Sprintf("SELECT * FROM users WHERE userid = '%s'", userid)).Scan(&data.UserID, &data.RegisteredOn, &data.LastLogin, &data.Username, &data.AvatarURL, &data.Xbox, &data.IP)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return UserData{}, nil
		}
		return UserData{}, err
	}

	return data, nil
}