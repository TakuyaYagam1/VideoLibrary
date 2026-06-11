package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App        App
	HTTP       HTTP
	PostgreSQL PostgreSQL
	Redis      Redis
	Cache      Cache
	SeaweedFS  SeaweedFS
	Log        Log
}

type App struct {
	Name string
	Env  string
}

type HTTP struct {
	Addr string
}

type PostgreSQL struct {
	DSN            string
	MaxConns       int
	MinConns       int
	RetryTimeout   time.Duration
	ConnectTimeout time.Duration
	MigrationsPath string
}

type Redis struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

type Cache struct {
	VideoListTTL time.Duration
}

type SeaweedFS struct {
	PublicURL string
}

type Log struct {
	Level    string
	Format   string
	Output   string
	FilePath string
}

type MissingRequiredEnvError struct {
	Names []string
}

func (e MissingRequiredEnvError) Error() string {
	return "missing required environment variables: " + strings.Join(e.Names, ", ")
}

func Load() (Config, error) {
	return LoadFromLookup(os.LookupEnv)
}

func LoadFromLookup(lookup func(string) (string, bool)) (Config, error) {
	required := []string{
		"APP_NAME",
		"APP_ENV",
		"HTTP_ADDR",
		"POSTGRES_DSN",
		"SEAWEEDFS_PUBLIC_URL",
		"LOG_LEVEL",
		"LOG_FORMAT",
	}

	values := make(map[string]string, len(required))
	var missing []string
	for _, name := range required {
		value, ok := lookup(name)
		value = strings.TrimSpace(value)
		if !ok || value == "" {
			missing = append(missing, name)
			continue
		}

		values[name] = value
	}

	redisHost, redisPort, redisMissing, err := redisEndpoint(lookup)
	if err != nil {
		return Config{}, err
	}
	missing = append(missing, redisMissing...)

	if len(missing) > 0 {
		sort.Strings(missing)
		return Config{}, MissingRequiredEnvError{Names: missing}
	}

	redisDB, err := optionalInt(lookup, "REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	redisPoolSize, err := optionalInt(lookup, "REDIS_POOL_SIZE", 0)
	if err != nil {
		return Config{}, err
	}
	redisMinIdleConns, err := optionalInt(lookup, "REDIS_MIN_IDLE_CONNS", 0)
	if err != nil {
		return Config{}, err
	}
	cacheVideoListTTL, err := optionalDuration(lookup, "CACHE_VIDEO_LIST_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	postgresMaxConns, err := optionalInt(lookup, "POSTGRES_MAX_CONNS", 0)
	if err != nil {
		return Config{}, err
	}
	postgresMinConns, err := optionalInt(lookup, "POSTGRES_MIN_CONNS", 0)
	if err != nil {
		return Config{}, err
	}
	postgresRetryTimeout, err := optionalDuration(lookup, "POSTGRES_RETRY_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	postgresConnectTimeout, err := optionalDuration(lookup, "POSTGRES_CONNECT_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		App: App{
			Name: values["APP_NAME"],
			Env:  values["APP_ENV"],
		},
		HTTP: HTTP{
			Addr: values["HTTP_ADDR"],
		},
		PostgreSQL: PostgreSQL{
			DSN:            values["POSTGRES_DSN"],
			MaxConns:       postgresMaxConns,
			MinConns:       postgresMinConns,
			RetryTimeout:   postgresRetryTimeout,
			ConnectTimeout: postgresConnectTimeout,
			MigrationsPath: optionalStringDefault(lookup, "POSTGRES_MIGRATIONS_PATH", "migrations"),
		},
		Redis: Redis{
			Host:         redisHost,
			Port:         redisPort,
			Password:     optionalString(lookup, "REDIS_PASSWORD"),
			DB:           redisDB,
			PoolSize:     redisPoolSize,
			MinIdleConns: redisMinIdleConns,
		},
		Cache: Cache{
			VideoListTTL: cacheVideoListTTL,
		},
		SeaweedFS: SeaweedFS{
			PublicURL: values["SEAWEEDFS_PUBLIC_URL"],
		},
		Log: Log{
			Level:    values["LOG_LEVEL"],
			Format:   values["LOG_FORMAT"],
			Output:   optionalStringDefault(lookup, "LOG_OUTPUT", "console"),
			FilePath: optionalString(lookup, "LOG_FILE_PATH"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if err := validateTCPAddr("HTTP_ADDR", c.HTTP.Addr); err != nil {
		return err
	}

	if err := validatePostgresDSN(c.PostgreSQL.DSN); err != nil {
		return err
	}
	if c.PostgreSQL.MinConns > c.PostgreSQL.MaxConns && c.PostgreSQL.MaxConns > 0 {
		return fmt.Errorf("POSTGRES_MIN_CONNS must be less than or equal to POSTGRES_MAX_CONNS")
	}
	if c.PostgreSQL.MigrationsPath == "" {
		return fmt.Errorf("POSTGRES_MIGRATIONS_PATH must not be empty")
	}

	if c.Redis.Port < 1 || c.Redis.Port > 65535 {
		return fmt.Errorf("REDIS_PORT must be in range 1-65535")
	}
	if c.Redis.PoolSize > 0 && c.Redis.MinIdleConns > c.Redis.PoolSize {
		return fmt.Errorf("REDIS_MIN_IDLE_CONNS must be less than or equal to REDIS_POOL_SIZE")
	}
	if c.Cache.VideoListTTL <= 0 {
		return fmt.Errorf("CACHE_VIDEO_LIST_TTL must be greater than 0")
	}

	if err := validateHTTPURL("SEAWEEDFS_PUBLIC_URL", c.SeaweedFS.PublicURL); err != nil {
		return err
	}

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}

	switch c.Log.Format {
	case "json", "text":
	default:
		return fmt.Errorf("LOG_FORMAT must be one of json, text")
	}

	switch c.Log.Output {
	case "console", "file", "both":
	default:
		return fmt.Errorf("LOG_OUTPUT must be one of console, file, both")
	}
	if (c.Log.Output == "file" || c.Log.Output == "both") && c.Log.FilePath == "" {
		return fmt.Errorf("LOG_FILE_PATH is required when LOG_OUTPUT is file or both")
	}

	return nil
}

func optionalString(lookup func(string) (string, bool), name string) string {
	value, _ := lookup(name)
	return strings.TrimSpace(value)
}

func optionalStringDefault(lookup func(string) (string, bool), name string, fallback string) string {
	value := optionalString(lookup, name)
	if value == "" {
		return fallback
	}

	return value
}

func optionalInt(lookup func(string) (string, bool), name string, fallback int) (int, error) {
	value, ok := lookup(name)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be greater than or equal to 0", name)
	}

	return parsed, nil
}

func redisEndpoint(lookup func(string) (string, bool)) (string, int, []string, error) {
	host := optionalString(lookup, "REDIS_HOST")
	portValue, portOK := lookup("REDIS_PORT")
	portValue = strings.TrimSpace(portValue)

	if host != "" && portOK && portValue != "" {
		port, err := parseInt("REDIS_PORT", portValue)
		return host, port, nil, err
	}

	if host == "" && (!portOK || portValue == "") {
		addr := optionalString(lookup, "REDIS_ADDR")
		if addr != "" {
			addrHost, addrPort, err := net.SplitHostPort(addr)
			if err != nil {
				return "", 0, nil, fmt.Errorf("REDIS_ADDR must be in host:port form: %w", err)
			}
			port, err := parseInt("REDIS_ADDR port", addrPort)
			return addrHost, port, nil, err
		}
	}

	var missing []string
	if host == "" {
		missing = append(missing, "REDIS_HOST")
	}
	if !portOK || portValue == "" {
		missing = append(missing, "REDIS_PORT")
	}
	if len(missing) > 0 {
		return "", 0, missing, nil
	}

	port, err := parseInt("REDIS_PORT", portValue)
	return host, port, nil, err
}

func parseInt(name string, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be greater than or equal to 0", name)
	}

	return parsed, nil
}

func optionalDuration(lookup func(string) (string, bool), name string, fallback time.Duration) (time.Duration, error) {
	value, ok := lookup(name)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", name)
	}

	return parsed, nil
}

func validateTCPAddr(name string, value string) error {
	if _, _, err := net.SplitHostPort(value); err != nil {
		return fmt.Errorf("%s must be in host:port form: %w", name, err)
	}

	return nil
}

func validatePostgresDSN(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("POSTGRES_DSN must be a valid URL: %w", err)
	}

	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return fmt.Errorf("POSTGRES_DSN must use postgres or postgresql scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("POSTGRES_DSN must include a host")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return fmt.Errorf("POSTGRES_DSN must include a database name")
	}

	return nil
}

func validateHTTPURL(name string, value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", name, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https scheme", name)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s must include a host", name)
	}

	return nil
}
