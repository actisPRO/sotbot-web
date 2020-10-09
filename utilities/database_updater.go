package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/logger"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Config struct {
	ClientId      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	ServerAddress string `json:"server_address"`
	DBHost        string `json:"db_host"`
	DBName        string `json:"db_name"`
	DBUser        string `json:"db_user"`
	DBPassword    string `json:"db_password"`
}

type UserData struct {
	UserID                string
	RegisteredOn          time.Time
	LastLogin             time.Time
	Username              string
	AvatarURL             string
	Xbox                  string
	IP                    string
	AccessToken           string
	RefreshToken          string
	AccessTokenExpiration time.Time
}

// This app updates users table
// At first it checks the token if it is expired (and refreshes it, if so)
// After that it refreshes all the data from the table
func main() {
	config := ReadConfig()
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", config.DBUser, config.DBPassword, config.DBHost, config.DBName))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	lf, err := os.OpenFile("database_updater.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		logger.Fatal("Failed to open log file: " + err.Error())
	}
	logger.Init("Logger", true, true, lf)

	rows, err := db.Query("SELECT * FROM users")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var data = UserData{}
		err := rows.Scan(&data.UserID, &data.RegisteredOn, &data.LastLogin, &data.Username, &data.AvatarURL, &data.Xbox, &data.IP, &data.AccessToken, &data.RefreshToken, &data.AccessTokenExpiration)
		if err != nil {
			panic(err)
		}
		logger.Info(fmt.Sprintf("Updating info for user %s (username in DB: %s) . . .", data.UserID, data.Username))

		// мы должны обновить токен, если он истёк
		// https://discord.com/developers/docs/topics/oauth2#authorization-code-grant-refresh-token-exchange-example
		if time.Now().After(data.AccessTokenExpiration) || time.Now() == data.AccessTokenExpiration {
			resp, err := http.PostForm("https://discord.com/api/oauth2/token", url.Values{
				"client_id":     {config.ClientId},
				"client_secret": {config.ClientSecret},
				"grant_type":    {"refresh_token"},
				"refresh_token": {data.RefreshToken},
				"redirect_uri":  {config.ServerAddress + "login"},
				"scope":         {"identify connections"},
			})
			if err != nil {
				panic(err)
			}

			var res map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&res)
			if err != nil {
				panic(err)
			}
			if res["error"] != nil {
				panic(res["error"])
			}

			data.AccessToken = res["access_token"].(string)
			data.RefreshToken = res["refresh_token"].(string)
			expiresIn := res["expires_in"].(float64)
			expiration := time.Now().Add(time.Second * time.Duration(expiresIn)) //.Format("2006-01-02 15:04:05")

			data.AccessTokenExpiration = expiration

			logger.Info("--- Successfully refreshed token");
		}

		// теперь получение данных каждого пользователя и обновление

		session, err := discordgo.New("Bearer " + data.AccessToken)
		if err != nil {
			panic(err)
		}
		user, err := session.User("@me")
		if err != nil {
			panic(err)
		}
		connections, err := session.UserConnections()
		if err != nil {
			panic(err)
		}

		logger.Info("--- Successfully got data from Discord")

		xboxConnection := ""
		for i := 0; i < len(connections); i++ {
			if connections[i].Type == "xbox" {
				xboxConnection = connections[i].Name
			}
		}

		data.Username = user.String()
		data.AvatarURL = user.AvatarURL("")
		data.Xbox = xboxConnection

		// проверим, есть ли в xboxes данный xbox-аккаунт и если нет - запишем
		if xboxConnection != "" { // но только если xbox привязан
			var xboxSql string
			err = db.QueryRow("SELECT xbox FROM xboxes WHERE xbox = ? AND userid = ?", xboxConnection, data.UserID).Scan(&xboxSql)
			if err != nil && err == sql.ErrNoRows { // отсутствуют строки
				_, _ = db.Exec("INSERT INTO xboxes(userid, xbox) VALUES (?, ?)", data.UserID, xboxConnection)
				logger.Info("--- Successfully updated xboxes table")
			}
		}

		// сохранение в users
		_, err = db.Query("UPDATE users SET username = ?, avatarurl = ?, xbox = ?, access_token = ?, refresh_token = ?, access_token_expiration = ? WHERE userid = ?", data.Username, data.AvatarURL, data.Xbox, data.AccessToken, data.RefreshToken, data.AccessTokenExpiration, data.UserID)
		if err != nil {
			panic(err)
		}
		logger.Info("--- Successfully updated users table.")
	}
}

// Считывает конфигурацию из conf.json
func ReadConfig() Config {
	config := Config{}

	jsonFile, err := os.Open("../conf.json")
	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)

	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		panic(err)
	}

	return config
}
