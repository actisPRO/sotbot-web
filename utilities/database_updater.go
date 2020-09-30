package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
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

		xboxConnection := ""
		for i := 0; i < len(connections); i++ {
			if connections[i].Type == "xbox" {
				xboxConnection = connections[i].Name
			}
		}

		data.Username = user.Username
		data.AvatarURL = user.AvatarURL("")
		data.Xbox = xboxConnection

		// сохранение в бд

		_, err = db.Query("UPDATE users SET username = ?, avatarurl = ?, xbox = ?, access_token = ?, refresh_token = ?, access_token_expiration = ? WHERE userid = ?", data.Username, data.AvatarURL, data.Xbox, data.AccessToken, data.RefreshToken, data.AccessTokenExpiration, data.UserID)
		if err != nil {
			panic(err)
		}
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
