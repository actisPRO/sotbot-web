package main

import (
	"./lib"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/google/logger"
	"github.com/gorilla/sessions"
	"html/template"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	indexTmpl := template.Must(template.ParseFiles("views/index.html"))
	errorTmpl := template.Must(template.ParseFiles("views/error.html"))

	session, _ := store.Get(r, "sotweb")
	user, err := lib.GetUserDataFromDB(db, session.Values["userid"].(string))
	if err != nil {
		err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

		return
	}

	member, _ := discord.GuildMember(config.Guild, user.UserID)
	accessLevel := lib.GetAccessLevelFromRoles(member, config)
	blacklisted := false
	if IsBlacklisted(user.UserID, "") || IsBlacklisted("", user.Xbox) {
		blacklisted = true
	}

	err = indexTmpl.Execute(w, lib.IndexData{
		Username:    user.Username,
		AccessLevel: accessLevel,
		Xbox:        user.Xbox,
		Blacklisted: blacklisted,
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	//session, _ := store.Get(r, "sotweb")
	errorTmpl := template.Must(template.ParseFiles("views/error.html"))
	loginTmpl := template.Must(template.ParseFiles("views/login.html"))
	redirectTmpl := template.Must(template.ParseFiles("views/redirect.html"))

	session, _ := store.Get(r, "sotweb")
	if session.Values["auth"] == true {
		err := redirectTmpl.Execute(w, lib.RedirectData{RedirectURL: "/"})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	}

	// we accept only GET requests
	if r.Method != "GET" {
		http.Error(w, "Login requires GET method", 403)
		return
	}

	q := r.URL.Query()
	// Если пользователь просто перешел на /login
	if q.Get("code") == "" && q.Get("error") == "" {
		err := loginTmpl.Execute(w, lib.LoginData{OAuth: config.DiscordOAuth})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		return
	}
	// Если Discord вернул ошибку
	if q.Get("error") != "" {
		data := lib.ErrorData{
			Message: "Discord вернул неизвестную ошибку. Пожалуйста, попробуйте снова.",
		}
		if q.Get("error") == "access_denied" {
			data.Message = "Discord отказал нам в авторизации. Пожалуйста, попробуйте снова."
		}

		err := errorTmpl.Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		return
	}
	// Если Discord вернул code - зарегистрируем пользователя или произведем авторизацию
	if q.Get("code") != "" {
		// Обмен кода на токены авторизации и обновления
		// Подробнее: https://discord.com/developers/docs/topics/oauth2#authorization-code-grant-redirect-url-example
		resp, err := http.PostForm("https://discord.com/api/oauth2/token", url.Values{
			"client_id":     {config.ClientId},
			"client_secret": {config.ClientSecret},
			"grant_type":    {"authorization_code"},
			"code":          {q.Get("code")},
			"redirect_uri":  {config.ServerAddress + "login"},
			"scope":         {"identify connections"},
		})
		if err != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при обмене токена. " + err.Error()})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}

		var res map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&res)
		if err != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при парсинге JSON. " + err.Error()})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
		}
		if res["error"] != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при запросе токена. " + res["error"].(string)})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}

		session, _ := store.Get(r, "sotweb")
		session.Values["auth"] = false
		_ = sessions.Save(r, w)

		accessToken := res["access_token"].(string)
		refreshToken := res["refresh_token"].(string)
		expiresIn := res["expires_in"].(float64)

		logger.Info(expiresIn)

		expiration := time.Now().Add(time.Second * time.Duration(expiresIn)).Format("2006-01-02 15:04:05")

		_, err = db.Exec(fmt.Sprintf("UPDATE sessions SET access_token = '%s', refresh_token = '%s' WHERE id = '%s'", accessToken, refreshToken, session.ID))
		if err != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}

		dSession, err := discordgo.New("Bearer " + accessToken)
		if err != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при соединении с Discord. " + err.Error()})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}
		dUser, _ := dSession.User("@me")

		connections, err := dSession.UserConnections()
		if err != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при соединении с Discord. " + err.Error()})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}

		xboxConnection := ""
		for i := 0; i < len(connections); i++ {
			if connections[i].Type == "xbox" {
				xboxConnection = connections[i].Name
			}
		}

		ip, err := lib.GetIP(r)
		if err != nil {
			ip = "unknown"
		}

		regDate := time.Now().Format("2006-01-02 15:04:05")

		// Проверим, зарегистрирован ли пользователь
		dbData, err := lib.GetUserDataFromDB(db, dUser.ID)
		if err != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}
		if dbData.UserID == "" { // не зарегистрирован
			_, err = db.Exec("INSERT INTO users(userid, registered_on, last_login, username, avatarurl, xbox, ip, access_token, refresh_token, access_token_expiration) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
				dUser.ID, regDate, regDate, dUser.String(), dUser.AvatarURL(""), xboxConnection, ip, accessToken, refreshToken, expiration)
			if err != nil {
				err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
				if err != nil {
					http.Error(w, err.Error(), 500)
				}
				return
			}

			_, err = IsTryingToBypassBlacklist(dUser.ID, xboxConnection, ip)
			if err != nil {
				errLogger.Error("Error during IsTryingToBypassBlacklist procedure: " + err.Error())
			}
		} else {
			_, _ = db.Exec("UPDATE users SET ip = ?, last_login = ?, access_token = ?, refresh_token = ?, access_token_expiration = ? WHERE userid = ?",
				ip, regDate, accessToken, refreshToken, expiration, dUser.ID)
		}

		session.Values["auth"] = true
		session.Values["userid"] = dUser.ID
		_ = session.Save(r, w)

		err = redirectTmpl.Execute(w, lib.RedirectData{RedirectURL: "/"})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	}
}

// Getting Xbox connection from user's account
func XboxHandler(w http.ResponseWriter, r *http.Request) {
	errorTmpl := template.Must(template.ParseFiles("views/error.html"))
	redirectTmpl := template.Must(template.ParseFiles("views/redirect.html"))

	session, _ := store.Get(r, "sotweb")
	token, err := lib.GetTokenFromSession(db, session)
	if err != nil {
		err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

		return
	}

	user, err := lib.GetUserDataFromDiscord(token)
	if err != nil {
		err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при получении ваших данных. " + err.Error()})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		return
	}

	if user.Xbox == "" {
		err := errorTmpl.Execute(w, lib.ErrorData{Message: "К твоему Discord-аккаунту не привязан Xbox. Пожалуйста, привяжи его и повтори попытку."})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		return
	}

	_, err = db.Exec(fmt.Sprintf("UPDATE users SET xbox='%s' WHERE userid='%s'", user.Xbox, user.UserID))
	if err != nil {
		err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		return
	}

	err = redirectTmpl.Execute(w, lib.RedirectData{RedirectURL: "/"})
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func BotControlPanelHandler(w http.ResponseWriter, r *http.Request) {
	errorTmpl := template.Must(template.ParseFiles("views/error.html"))
	botcpTmpl := template.Must(template.ParseFiles("views/botcp.html"))

	session, _ := store.Get(r, "sotweb")
	user, err := lib.GetUserDataFromDB(db, session.Values["userid"].(string))
	if err != nil {
		err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

		return
	}
	member, _ := discord.GuildMember(config.Guild, user.UserID)
	accessLevel := lib.GetAccessLevelFromRoles(member, config)

	if accessLevel < lib.Admin {
		err := errorTmpl.Execute(w, lib.ErrorData{Message: "У вас недостаточно прав для просмотра данной страницы"})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

		return
	}

	if r.Method == "GET" {
		status := "произошла ошибка при запросе статуса"
		out, err := exec.Command("systemctl", "is-active", "sotbot").Output()

		if err == nil {
			status = string(out[:])
		}

		err = botcpTmpl.Execute(w, lib.BotCPData{Status: status})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	} else if r.Method == "POST" {
		_ = r.ParseForm()
		if r.PostFormValue("action") == "" {
			_, _ = fmt.Fprint(w, "{ \"error\": \"no action specified\" }")
			return
		} else {
			status := "произошла ошибка при обновлении статуса"
			switch r.PostFormValue("action") {
			case "start":
				_ = exec.Command("sudo", "systemctl", "start", "sotbot.service")
			case "restart":
				_ = exec.Command("sudo", "systemctl", "restart", "sotbot.service")
			case "stop":
				_ = exec.Command("sudo", "systemctl", "stop", "sotbot.service")
			}

			out, err := exec.Command("systemctl", "is-active", "sotbot").Output()
			if err == nil {
				status = string(out[:])
			}
			_, _ = fmt.Fprintf(w, "{ \"status\": \"%s\" }", status)
			return
		}
	}
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "sotweb")
	session.Values["auth"] = false
	_ = session.Save(r, w)

	redirectTmpl := template.Must(template.ParseFiles("views/redirect.html"))
	err := redirectTmpl.Execute(w, lib.RedirectData{RedirectURL: "/login"})
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func BlacklistHandler(w http.ResponseWriter, r *http.Request) {
	blacklistTmpl := template.Must(template.ParseFiles("views/blacklist.html"))
	var blacklistEntries []lib.BlacklistEntry

	rows, err := db.Query("SELECT * FROM blacklist ORDER BY ban_date")
	if err != nil {
		errLogger.Error("Error in query 'SELECT * FROM blacklist ORDER BY ban_date': " + err.Error())
		http.Error(w, "Unable to get rows from the blacklist DB", 500)
		return
	}
	defer rows.Close()

	var moderators = map[string]string {}
	for rows.Next() {
		entry := lib.BlacklistEntry{}
		err = rows.Scan(&entry.Id, &entry.DiscordId, &entry.DiscordUsername, &entry.Xbox, &entry.Date, &entry.Moderator,
			&entry.Reason, &entry.Additional)
		if err != nil {
			errLogger.Error("Error while scanning a blacklist entry: " + err.Error())
			http.Error(w, "Error while scanning a blacklist entry", 500)
			return
		}
		// discord_id might be 0 (as the bot stores it as ulong), so we should change it to an empty string
		if entry.DiscordId == "0" {
			entry.DiscordId = ""
		}
		entry.DateString = entry.Date.Format("02.01.2006")
		entry.AdditionalName = "null"
		if entry.Additional.String != "" {
			entry.AdditionalName = lib.GetWebsiteName(entry.Additional.String)
		}

		// check if moderator is in the map, if not - get his name
		_, mKnown := moderators[entry.Moderator]
		if !mKnown {
			mUser, err := discord.User(entry.Moderator)
			if err != nil {
				entry.ModeratorName = "ID: " + entry.Moderator
			} else {
				entry.ModeratorName = mUser.String()
				moderators[entry.Moderator] = mUser.String()
			}
		} else {
			entry.ModeratorName = moderators[entry.Moderator]
		}

		blacklistEntries = append(blacklistEntries, entry)
	}

	err = blacklistTmpl.Execute(w, lib.BlacklistData{Entries: blacklistEntries})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

// use only one param, the second must be ""
func IsBlacklisted(userid string, xbox string) bool {
	var id string
	var query string
	var param string
	if userid != "" {
		query = "SELECT id FROM blacklist WHERE discord_id = ?"
		param = userid
	} else if xbox != "" {
		query = "SELECT id FROM blacklist WHERE xbox = ?"
		param = xbox
	} else {
		return false
	}

	err := db.QueryRow(query, param).Scan(&id)
	if err == nil {
		return true
	} else {
		if err != sql.ErrNoRows {
			errLogger.Error(fmt.Sprintf("SQL error (query: %s): %s", query, err.Error()))
		}
		return false
	}
}

/*
	Checks if user is trying to bypass the blacklist using a twink account.
	If true blocks user
*/
func IsTryingToBypassBlacklist(userid string, xbox string, ip string) (bool, error) {
	var err error
	if IsBlacklisted(userid, "") {
		// User ID is already is in the blacklist, but his Xbox is new
		if !IsBlacklisted("", xbox) && xbox != "" {
			err = AddToBlacklist(userid, xbox)
			if err != nil {
				return true, err
			}
			return true, nil
		}
		return false, nil
	}

	if IsBlacklisted("", xbox) {
		// Xbox is already in the blacklist, but user ID is new.
		if !IsBlacklisted(userid, "") {
			err = AddToBlacklist(userid, xbox)
			if err != nil {
				return true, err
			}
			return true, nil
		}
		return false, nil
	}

	var id string
	err = db.QueryRow("SELECT id FROM blacklist WHERE discord_id = (SELECT userid FROM ips WHERE ip = ?)", ip).Scan(&id)
	if err == nil {
		//new user id and xbox, but an old ip
		err = AddToBlacklist(userid, xbox)
		if err != nil {
			return true, err
		}
		return true, nil
	} else {
		if err != sql.ErrNoRows {
			return true, err
		}
		return false, nil
	}
}

func AddToBlacklist(userid string, xbox string) error {
	user, err := discord.User(userid)
	if err != nil {
		return err
	}
	bot, err := discord.User("@me")
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO blacklist(id, discord_id, discord_username, xbox, ban_date, moderator_id, reason) VALUES (?, ?, ?, ?, ?, ?, ?)",
		lib.RandomString(6), userid, user.String(), xbox, time.Now().Format("2006-01-02"), bot.ID, "Автоматическая блокировка системой защиты")
	if err != nil {
		return err
	}

	return nil
}