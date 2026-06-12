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

const (
	backendBindHost    = "0.0.0.0"
	postgresDockerHost = "postgres"
	redisDockerHost    = "redis"
)

type Config struct {
	App        App
	HTTP       HTTP
	PostgreSQL PostgreSQL
	Redis      Redis
	Cache      Cache
	Health     Health
	SeaweedFS  SeaweedFS
	Log        Log
}

type App struct {
	Name string
	Env  string
}

type HTTP struct {
	Port              int
	Addr              string
	AllowedOrigins    []string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
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

type Health struct {
	CheckTimeout time.Duration
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
		"BACKEND_PORT",
		"FRONTEND_DOMAIN",
		"POSTGRES_DB",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_PORT",
		"POSTGRES_SSLMODE",
		"SEAWEEDFS_FILES_DOMAIN",
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

	backendPort, err := parsePort("BACKEND_PORT", values["BACKEND_PORT"])
	if err != nil {
		return Config{}, err
	}
	backendAddr := net.JoinHostPort(backendBindHost, strconv.Itoa(backendPort))
	frontendOrigin, err := publicHTTPSURL("FRONTEND_DOMAIN", values["FRONTEND_DOMAIN"])
	if err != nil {
		return Config{}, err
	}

	postgresPort, err := parsePort("POSTGRES_PORT", values["POSTGRES_PORT"])
	if err != nil {
		return Config{}, err
	}
	postgresDSN := buildPostgresDSN(
		values["POSTGRES_USER"],
		values["POSTGRES_PASSWORD"],
		postgresDockerHost,
		postgresPort,
		values["POSTGRES_DB"],
		values["POSTGRES_SSLMODE"],
	)

	seaweedfsPublicURL, err := publicHTTPSURL("SEAWEEDFS_FILES_DOMAIN", values["SEAWEEDFS_FILES_DOMAIN"])
	if err != nil {
		return Config{}, err
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
	healthCheckTimeout, err := optionalDuration(lookup, "HEALTH_CHECK_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpReadHeaderTimeout, err := optionalDuration(lookup, "HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpReadTimeout, err := optionalDuration(lookup, "HTTP_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpWriteTimeout, err := optionalDuration(lookup, "HTTP_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpIdleTimeout, err := optionalDuration(lookup, "HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpShutdownTimeout, err := optionalDuration(lookup, "HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpMaxHeaderBytes, err := optionalInt(lookup, "HTTP_MAX_HEADER_BYTES", 1<<20)
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
			Port:              backendPort,
			Addr:              backendAddr,
			AllowedOrigins:    []string{frontendOrigin},
			ReadTimeout:       httpReadTimeout,
			ReadHeaderTimeout: httpReadHeaderTimeout,
			WriteTimeout:      httpWriteTimeout,
			IdleTimeout:       httpIdleTimeout,
			ShutdownTimeout:   httpShutdownTimeout,
			MaxHeaderBytes:    httpMaxHeaderBytes,
		},
		PostgreSQL: PostgreSQL{
			DSN:            postgresDSN,
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
		Health: Health{
			CheckTimeout: healthCheckTimeout,
		},
		SeaweedFS: SeaweedFS{
			PublicURL: seaweedfsPublicURL,
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
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("BACKEND_PORT must be in range 1-65535")
	}
	if err := validateTCPAddr("BACKEND_ADDR", c.HTTP.Addr); err != nil {
		return err
	}
	if c.HTTP.ReadTimeout <= 0 {
		return fmt.Errorf("HTTP_READ_TIMEOUT must be greater than 0")
	}
	if c.HTTP.ReadHeaderTimeout <= 0 {
		return fmt.Errorf("HTTP_READ_HEADER_TIMEOUT must be greater than 0")
	}
	if c.HTTP.WriteTimeout <= 0 {
		return fmt.Errorf("HTTP_WRITE_TIMEOUT must be greater than 0")
	}
	if c.HTTP.IdleTimeout <= 0 {
		return fmt.Errorf("HTTP_IDLE_TIMEOUT must be greater than 0")
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		return fmt.Errorf("HTTP_SHUTDOWN_TIMEOUT must be greater than 0")
	}
	if c.HTTP.MaxHeaderBytes <= 0 {
		return fmt.Errorf("HTTP_MAX_HEADER_BYTES must be greater than 0")
	}

	if err := validatePostgresURL(c.PostgreSQL.DSN); err != nil {
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
	if c.Health.CheckTimeout <= 0 {
		return fmt.Errorf("HEALTH_CHECK_TIMEOUT must be greater than 0")
	}

	if err := validateHTTPURL("seaweedfs public URL", c.SeaweedFS.PublicURL); err != nil {
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
	portValue, portOK := lookup("REDIS_PORT")
	portValue = strings.TrimSpace(portValue)

	if portOK && portValue != "" {
		port, err := parsePort("REDIS_PORT", portValue)
		return redisDockerHost, port, nil, err
	}

	var missing []string
	if !portOK || portValue == "" {
		missing = append(missing, "REDIS_PORT")
	}
	if len(missing) > 0 {
		return "", 0, missing, nil
	}

	port, err := parsePort("REDIS_PORT", portValue)
	return redisDockerHost, port, nil, err
}

func buildPostgresDSN(user string, password string, host string, port int, db string, sslmode string) string {
	postgresURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   db,
	}
	query := postgresURL.Query()
	query.Set("sslmode", sslmode)
	postgresURL.RawQuery = query.Encode()

	return postgresURL.String()
}

func publicHTTPSURL(name string, domain string) (string, error) {
	if err := validateDomain(name, domain); err != nil {
		return "", err
	}
	httpURL := url.URL{
		Scheme: "https",
		Host:   domain,
	}

	return httpURL.String(), nil
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

func parsePort(name string, value string) (int, error) {
	port, err := parseInt(name, value)
	if err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be in range 1-65535", name)
	}

	return port, nil
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

func validateDomain(name string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if strings.Contains(value, "://") {
		return fmt.Errorf("%s must be a domain name without scheme", name)
	}
	if strings.ContainsAny(value, "/?# \t\r\n") {
		return fmt.Errorf("%s must be a domain name without path, query, or whitespace", name)
	}

	return nil
}

func validatePostgresURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("postgres URL must be valid: %w", err)
	}

	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return fmt.Errorf("postgres URL must use postgres or postgresql scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("postgres URL must include a host")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return fmt.Errorf("postgres URL must include a database name")
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
