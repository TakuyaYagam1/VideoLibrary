package config

type Config struct {
	AppName string
}

func Default() Config {
	return Config{
		AppName: "videolibrary",
	}
}
