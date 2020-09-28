package main

import (
	"./lib"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/sessions"
	"html/template"
	"net/http"
	"net/url"
	"time"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "OK")
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	//session, _ := store.Get(r, "sotweb")
	errorTmpl := template.Must(template.ParseFiles("views/error.html"))
	loginTmpl := template.Must(template.ParseFiles("views/login.html"))

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
			"client_id": {config.ClientId},
			"client_secret": {config.ClientSecret},
			"grant_type": {"authorization_code"},
			"code": {q.Get("code")},
			"redirect_uri": {config.ServerAddress + "login"},
			"scope": {"identify connections"},
		})
		if err != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при обмене токена. " + err.Error()})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}

		var res map[string] interface {}
		err = json.NewDecoder(resp.Body).Decode(&res)
		if err != nil {
			err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при парсинге JSON. " + err.Error()})
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
		}

		session, _ := store.Get(r, "sotweb")
		session.Values["auth"] = false
		_ = sessions.Save(r, w)

		accessToken := res["access_token"].(string)
		refreshToken := res["refresh_token"].(string)

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
			_, err = db.Exec(fmt.Sprintf("INSERT INTO users(userid, registered_on, last_login, username, avatarurl, xbox, ip) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s')", dUser.ID, regDate, regDate, dUser.String(), dUser.AvatarURL(""), xboxConnection, ip))
			if err != nil {
				err := errorTmpl.Execute(w, lib.ErrorData{Message: "Ошибка при отправке запроса к БД. " + err.Error()})
				if err != nil {
					http.Error(w, err.Error(), 500)
				}
				return
			}
		} else {
			_, _ = db.Exec(fmt.Sprintf("UPDATE users SET ip = '%s', last_login = '%s' WHERE userid = '%s'", ip, regDate, dUser.ID))
		}

		session.Values["auth"] = true
		_ = session.Save(r, w)

		redirectTmpl := template.Must(template.ParseFiles("views/redirect.html"))
		err = redirectTmpl.Execute(w, lib.RedirectData{RedirectURL: "/"})
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	}
}