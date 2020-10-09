package main

import (
	"./lib"
	"database/sql"
	"fmt"
	"github.com/google/logger"
	"html/template"
	"net/http"
	"strings"
	"time"
)

var noAuthPages = []string {
	"login",
	"static",
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := lib.GetIP(r)
		if err != nil {
			ip = "unknown"
		}
		logger.Info(fmt.Sprintf("Request to %s (method %s) from %s", r.RequestURI, r.Method, ip))
		next.ServeHTTP(w, r)
	})
}

// проверка на авторизацию
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// страницы без авторизации
		for i := 0; i < len(noAuthPages); i++ {
			if strings.Contains(r.RequestURI, noAuthPages[i]) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// переправление неавторизованных пользователей на /login
		session, _ := store.Get(r, "sotweb")
		if session.Values["auth"] == nil || session.Values["auth"] == false {
			session.Values["auth"] = false;
			_ = session.Save(r, w)

			data := lib.RedirectData{RedirectURL: "/login"}
			tmpl := template.Must(template.ParseFiles("views/redirect.html"))

			err := tmpl.Execute(w, data)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

func ipLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// страницы без авторизации
		for i := 0; i < len(noAuthPages); i++ {
			if strings.Contains(r.RequestURI, noAuthPages[i]) {
				next.ServeHTTP(w, r)
				return
			}
		}

		session, _ := store.Get(r, "sotweb")
		user := session.Values["userid"].(string)
		timeNow := time.Now().Format("2006-01-02 15:04:05")
		ip, err := lib.GetIP(r)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		var userSql string
		err = db.QueryRow("SELECT userid FROM ips WHERE ip = ? AND userid = ?", ip, user).Scan(&userSql)
		if err != nil {
			if err == sql.ErrNoRows {
				// это значит, что данный ip не был записан в бд и нам нужно его записать
				_, err = db.Exec("INSERT INTO ips(userid, ip, last_used) VALUES(?, ?, ?)", user, ip, timeNow)
			} else {
				next.ServeHTTP(w, r)
				return
			}
		} else {
			// ip адрес уже есть в базе, обновим дату последнего запроса
			_, err = db.Exec("UPDATE ips SET last_used = ? WHERE userid = ? AND ip = ?", timeNow, user, ip)
		}

		//обновление ip пользователя в users
		_, err = db.Exec("UPDATE users SET ip = ? WHERE userid = ?", ip, user)

		next.ServeHTTP(w, r)
		return
	})
}