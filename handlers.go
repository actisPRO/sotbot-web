package main

import (
	"./lib"
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

	err = indexTmpl.Execute(w, lib.IndexData{
		Username:    user.Username,
		AccessLevel: accessLevel,
		Xbox:        user.Xbox,
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
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при запросе токена."})
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
			_, err = db.Exec(fmt.Sprintf("INSERT INTO users(userid, registered_on, last_login, username, avatarurl, xbox, ip, access_token, refresh_token, access_token_expiration) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s')", dUser.ID, regDate, regDate, dUser.String(), dUser.AvatarURL(""), xboxConnection, ip, accessToken, refreshToken, expiration))
			if err != nil {
				err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
				if err != nil {
					http.Error(w, err.Error(), 500)
				}
				return
			}
		} else {
			_, _ = db.Exec(fmt.Sprintf("UPDATE users SET ip = '%s', last_login = '%s', access_token = '%s', refresh_token = '%s', access_token_expiration = '%s' WHERE userid = '%s'", ip, regDate, accessToken, refreshToken, expiration, dUser.ID))
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

// Привязка Xbox (доступна, если при регистрации не был привязан Xbox)
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
		out, err := exec.Command("systemctl is-active sotbot").Output()
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
				_ = exec.Command("sudo systemctl start sotbot")
			case "restart":
				_ = exec.Command("sudo systemctl restart sotbot")
			case "stop":
				_ = exec.Command("sudo systemctl stop sotbot")
			}

			out, err := exec.Command("systemctl is-active sotbot").Output()
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
