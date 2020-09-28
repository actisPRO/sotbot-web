package lib

type LoginData struct {
	Status 	bool
	OAuth 	string
}

type ErrorData struct {
	Message string
}

type RedirectData struct {
	RedirectURL	string
}