package lib

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Configuration struct {
	AuthKey       	string 		`json:"auth_key"`		// Ключ для хранилища сессий
	ClientId      	string 		`json:"client_id"`		// ID приложения Discord
	ClientSecret  	string 		`json:"client_secret"`	// Ключ приложения Discord
	DiscordOAuth  	string 		`json:"discord_oauth"`	// Ссылка на авторизацию через Discord
	ServerAddress 	string 		`json:"server_address"`	// Адрес сервера
	DBHost			string		`json:"db_host"`		// Адрес сервера БД
	DBName 			string		`json:"db_name"`		// Имя БД
	DBUser 			string		`json:"db_user"`		// Пользователь БД
	DBPassword 		string		`json:"db_password"`	// Пароль пользователя БД
	BotToken		string		`json:"bot_token"`		// Токен бота
	Guild			string		`json:"guild"`			// ID Discord-сервера
	AdminRoles		[]string	`json:"admin_roles"`	// Массив с ID ролей с доступом администратора
	ModRoles		[]string	`json:"mod_roles"`		// Массив с ID ролей с доступом модератора
	CaptainRoles	[]string	`json:"captain_roles"`	// Массив с ID ролей с доступом капитана рейда
}

// Считывает конфигурацию из conf.json
func ReadConfig() (Configuration, error) {
	config := Configuration{}

	jsonFile, err := os.Open("conf.json")
	if err != nil {
		return config, fmt.Errorf("Unable to read conf.json: " + err.Error())
	}
	byteValue, _ := ioutil.ReadAll(jsonFile)

	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		return config, fmt.Errorf("Unable to unmarshal conf.json: " + err.Error())
	}

	return config, nil
}