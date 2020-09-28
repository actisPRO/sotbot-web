package main

import (
	"./lib"
	"html/template"
	"log"
	"net/http"
	"strings"
)

var noAuthPages = []string {
	"login",
	"public",
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := lib.GetIP(r)
		if err != nil {
			ip = "unknown"
		}
		log.Println("Request to " + r.RequestURI + " from " + ip)
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