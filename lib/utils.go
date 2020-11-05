package lib

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
)

const letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GetIP(r *http.Request) (string, error) {
	//Get IP from CF-Connecting-IP (because we are using Cloudflare)
	ip := r.Header.Get("CF-Connecting-IP")
	netIP := net.ParseIP(ip)
	if netIP != nil {
		return ip, nil
	}

	//Get IP from X-FORWARDED-FOR header
	ips := r.Header.Get("X-Forwarded-For")
	splitIps := strings.Split(ips, ",")
	for _, ip := range splitIps {
		netIP := net.ParseIP(ip)
		if netIP != nil {
			return ip, nil
		}
	}

	//Get IP from RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	netIP = net.ParseIP(ip)
	if netIP != nil {
		return ip, nil
	}
	return "", fmt.Errorf("No valid ip found")
}

// weird way to do that but who cares
func GetWebsiteName(url string) string {
	if strings.Contains(url, "imgur.com") {
		return "Imgur"
	} else if strings.Contains(url, "youtube.com") {
		return "YouTube"
	} else if strings.Contains(url, "cdn.discordapp.com") {
		return "Discord CDN"
	} else {
		return "Сторонний сайт"
	}
}

func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}