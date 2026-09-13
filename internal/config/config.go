package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ShortURLBaseURL string
	Environment     string
	Host            string
	Port            int
	AppKey          string
	LogLevel        string
	DBHost          string
	DBPort          uint16
	DBUser          string
	DBPassword      string
	DBName          string
	DBSchema        string
	GoogleClientID  string
	Origins         []string
	DriveDisk       string
	DriveKey        string
	DriveSecret     string
	DriveEndpoint   string
	DriveBucket     string
	DriveRegion     string
	Location        *time.Location
}

func Load() (Config, error) {
	port, err := strconv.Atoi(value("PORT", "3334"))
	if err != nil || port != 3334 {
		return Config{}, errors.New("PORT must remain 3334")
	}
	dbPort, err := strconv.ParseUint(value("DB_PORT", "5432"), 10, 16)
	if err != nil {
		return Config{}, errors.New("invalid DB_PORT")
	}
	schema := value("DB_SCHEMA", "public")
	if !regexp.MustCompile(`^[a-z_][a-z0-9_]*$`).MatchString(schema) {
		return Config{}, errors.New("invalid DB_SCHEMA")
	}
	location := time.Local
	if tz := os.Getenv("TZ"); tz != "" {
		location, err = time.LoadLocation(tz)
		if err != nil {
			return Config{}, errors.New("invalid TZ")
		}
	}
	c := Config{Environment: value("NODE_ENV", "development"), Host: value("HOST", "127.0.0.1"), Port: port,
		AppKey: os.Getenv("APP_KEY"), LogLevel: value("LOG_LEVEL", "info"), DBHost: os.Getenv("DB_HOST"),
		DBPort: uint16(dbPort), DBUser: os.Getenv("DB_USER"), DBPassword: os.Getenv("DB_PASSWORD"), DBName: os.Getenv("DB_DATABASE"), DBSchema: schema,
		GoogleClientID: os.Getenv("GOOGLE_CLIENT_ID"), DriveDisk: os.Getenv("DRIVE_DISK"), DriveKey: os.Getenv("DRIVE_ACCESS_KEY_ID"),
		DriveSecret: os.Getenv("DRIVE_SECRET_ACCESS_KEY"), DriveEndpoint: os.Getenv("DRIVE_ENDPOINT"), DriveBucket: os.Getenv("DRIVE_BUCKET"), DriveRegion: os.Getenv("DRIVE_REGION"), Location: location}
	for _, origin := range strings.Split(value("ADMIN_CORS_ORIGINS", "http://localhost:3005"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			c.Origins = append(c.Origins, origin)
		}
	}
	if len(c.AppKey) < 16 || c.DBHost == "" || c.DBUser == "" || c.DBName == "" {
		return Config{}, errors.New("APP_KEY and database configuration are required")
	}
	shortBase := "http://localhost:4000"
	if c.Environment == "production" {
		shortBase = "https://s.salmanitb.com"
	}
	c.ShortURLBaseURL = strings.TrimRight(value("SHORT_URL_BASE_URL", shortBase), "/")
	u, err := url.Parse(c.ShortURLBaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return Config{}, errors.New("invalid SHORT_URL_BASE_URL")
	}
	return c, nil
}

func (c Config) Address() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }
func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
