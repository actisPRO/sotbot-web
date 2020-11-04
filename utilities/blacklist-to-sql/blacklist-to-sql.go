package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/logger"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
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
		return
	}
	logger.Init("Logger", true, true, lf)

	csvFile, err := os.Open("blacklist.csv")
	if err != nil {
		logger.Fatal("Failed to open a csv file " + err.Error())
		return
	}

	blReader := csv.NewReader(csvFile)
	blEntries, err := blReader.ReadAll()
	if err != nil {
		logger.Fatal("Failed to read a csv file " + err.Error())
		return
	}

	for i := 0; i < len(blEntries); i++ {
		discordUsername := blEntries[i][0]
		if blEntries[i][0] == "-" {
			discordUsername = ""
		}

		discordId := blEntries[i][1]
		if discordId == "-" {
			discordId = "0"
		}

		xbox := blEntries[i][2]
		if xbox == "-" {
			xbox = ""
		}

		dateTime, _ := time.Parse("02.01.2006", blEntries[i][3])
		date := dateTime.Format("2006-01-02")

		moderatorName := blEntries[i][4]
		var moderator string
		if strings.Contains(moderatorName, "yTkOo") {
			moderator = "352865785984712706"
		} else if strings.Contains(moderatorName, "Daniil Funny") {
			moderator = "358525134807498753"
		} else if strings.Contains(moderatorName, "Owland") {
			moderator = "257929409112178689"
		} else if strings.Contains(moderatorName, "Actis") {
			moderator = "261137595965243393"
		} else if strings.Contains(strings.ToLower(moderatorName), "bzmonk") {
			moderator = "260796436373831681"
		} else if strings.Contains(moderatorName, "GrayUr") {
			moderator = "200620229305303040"
		} else if strings.Contains(moderatorName, "Aquarius") {
			moderator = "282215661294583809"
		} else if strings.Contains(moderatorName, "Pechall") {
			moderator = "282215661294583809"
		} else {
			moderator = "437301228251250699"
		} // пиздец какой-то

		reason := blEntries[i][6]
		additional := blEntries[i][7]

		id := RandomString(6)

		_, err := db.Exec("INSERT INTO blacklist(id, discord_id, discord_username, xbox, ban_date, moderator_id, reason, additional) VALUES (?, ?, ?, ?, ?, ?, ?, ?);",
			id, discordId, discordUsername, xbox, date, moderator, reason, additional)
		if err != nil {
			logger.Error("Error while querying: " + err.Error())
			return
		}

		logger.Info(fmt.Sprintf("Finished saving string %d to the db.", i))
	}
}

const letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// Считывает конфигурацию из conf.json
func ReadConfig() Config {
	config := Config{}

	jsonFile, err := os.Open("../../conf.json")
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
