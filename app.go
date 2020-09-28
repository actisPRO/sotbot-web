package main

import (
	"./lib"
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/srinathgs/mysqlstore"
	"log"
	"net/http"
)

var (
	config	lib.Configuration
	discord	*discordgo.Session
	db		*sql.DB
	store	*mysqlstore.MySQLStore
)

func main() {
	var err error

	// Loading configuration
	config, err = lib.ReadConfig()
	if err != nil {
		panic(err)
	}
	log.Println("Successfully loaded configuration")

	store, err = mysqlstore.NewMySQLStore(fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true&loc=Local", config.DBUser, config.DBPassword, config.DBHost, config.DBName), "sessions", "/",604800, []byte(config.AuthKey))
	if err != nil {
		panic(err)
	}

	// Connecting to database
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", config.DBUser, config.DBPassword, config.DBHost, config.DBName))
	if err != nil {
		panic(err)
	}
	log.Println("Successfully connected to DB")

	// Starting up Discord bot
	discord, err = discordgo.New("Bot " + config.BotToken)
	if err != nil {
		panic(err)
	}
	err = discord.Open()
	if err != nil {
		panic(err)
	}
	log.Println("Successfully connected to Discord")

	// Set up HTTP server
	r := mux.NewRouter()
	r.HandleFunc("/", IndexHandler)
	r.HandleFunc("/login", LoginHandler)

	s := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))
	r.PathPrefix("/static/").Handler(s)

	r.Use(loggingMiddleware)
	r.Use(authMiddleware)

	http.Handle("/", r)
	log.Println("HTTP server is listening")
	err = http.ListenAndServe(":80", nil)
	if err != nil {
		panic(err)
	}
}
