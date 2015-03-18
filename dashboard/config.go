package main

import (
	"log"
	"os"
	"path"

	"github.com/flynn/flynn/Godeps/_workspace/src/github.com/gorilla/sessions"
)

type Config struct {
	Addr               string
	DefaultRouteDomain string
	ControllerDomain   string
	ControllerKey      string
	URL                string
	InterfaceURL       string
	PathPrefix         string
	CookiePath         string
	SecureCookies      bool
	LoginToken         string
	GithubToken        string
	SessionStore       *sessions.CookieStore
	AppName            string
	CACert             []byte
}

func LoadConfigFromEnv() *Config {
	conf := &Config{}
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}
	conf.Addr = ":" + port

	conf.DefaultRouteDomain = os.Getenv("DEFAULT_ROUTE_DOMAIN")
	if conf.DefaultRouteDomain == "" {
		log.Fatal("DEFAULT_ROUTE_DOMAIN is required!")
	}

	conf.ControllerDomain = os.Getenv("CONTROLLER_DOMAIN")
	if conf.ControllerDomain == "" {
		log.Fatal("CONTROLLER_DOMAIN is required!")
	}

	conf.ControllerKey = os.Getenv("CONTROLLER_KEY")
	if conf.ControllerKey == "" {
		log.Fatal("CONTROLLER_KEY is required!")
	}

	conf.URL = os.Getenv("URL")
	if conf.URL == "" {
		log.Fatal("URL is required!")
	}
	conf.InterfaceURL = conf.URL

	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET is required!")
	}
	conf.SessionStore = sessions.NewCookieStore([]byte(sessionSecret))

	pathPrefix := path.Clean(os.Getenv("PATH_PREFIX"))
	conf.CookiePath = pathPrefix
	if pathPrefix == "/" || pathPrefix == "." {
		pathPrefix = ""
		conf.CookiePath = "/"
	}
	conf.PathPrefix = pathPrefix

	conf.SecureCookies = os.Getenv("SECURE_COOKIES") != ""

	conf.LoginToken = os.Getenv("LOGIN_TOKEN")
	if conf.LoginToken == "" {
		log.Fatal("LOGIN_TOKEN is required")
	}

	conf.GithubToken = os.Getenv("GITHUB_TOKEN")

	conf.AppName = os.Getenv("APP_NAME")
	if conf.AppName == "" {
		conf.AppName = "dashboard"
	}

	conf.CACert = []byte(os.Getenv("CA_CERT"))

	return conf
}
