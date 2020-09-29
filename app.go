package main

import (
	"./lib"
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/logger"
	"github.com/gorilla/mux"
	"github.com/srinathgs/mysqlstore"
	"net/http"
	"os"
)

var (
	config  lib.Configuration
	discord *discordgo.Session
	db      *sql.DB
	store   *mysqlstore.MySQLStore
)

const logPath = "app.log"

func main() {
	var err error

	// Set up logger
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		logger.Fatal("Failed to open log file: " + err.Error())
	}
	logger.Init("Logger", true, true, lf)

	// Loading configuration
	config, err = lib.ReadConfig()
	if err != nil {
		panic(err)
	}
	logger.Info("Successfully loaded configuration")

	store, err = mysqlstore.NewMySQLStore(fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true&loc=Local", config.DBUser, config.DBPassword, config.DBHost, config.DBName), "sessions", "/", 604800, []byte(config.AuthKey))
	if err != nil {
		panic(err)
	}

	// Connecting to database
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", config.DBUser, config.DBPassword, config.DBHost, config.DBName))
	if err != nil {
		panic(err)
	}
	logger.Info("Successfully connected to DB")

	// Starting up Discord bot
	discord, err = discordgo.New("Bot " + config.BotToken)
	if err != nil {
		panic(err)
	}
	err = discord.Open()
	if err != nil {
		panic(err)
	}
	logger.Info("Successfully connected to Discord")

	// Set up HTTP server
	r := mux.NewRouter()
	r.HandleFunc("/", IndexHandler)
	r.HandleFunc("/login", LoginHandler)
	r.HandleFunc("/xbox", XboxHandler)
	r.HandleFunc("/logout", LogoutHandler)

	s := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))
	r.PathPrefix("/static/").Handler(s)

	r.Use(loggingMiddleware)
	r.Use(authMiddleware)

	http.Handle("/", r)
	logger.Info("HTTP server is listening")
	err = http.ListenAndServe(":9900", nil)
	if err != nil {
		panic(err)
	}
}
