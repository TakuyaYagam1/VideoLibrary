package app

import (
	"fmt"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func NewLogger(cfg config.Config) (logkit.Logger, error) {
	level, err := logLevel(cfg.Log.Level)
	if err != nil {
		return nil, err
	}

	output, err := logOutput(cfg.Log.Output)
	if err != nil {
		return nil, err
	}

	opts := []logkit.Option{
		logkit.WithLevel(level),
		logkit.WithOutput(output),
		logkit.WithServiceName(cfg.App.Name),
	}
	if output == logkit.FileOutput || output == logkit.BothOutput {
		opts = append(opts, logkit.WithFileOptions(logkit.FileOptions{
			Filename: cfg.Log.FilePath,
		}))
	}

	logger, err := logkit.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	return logger, nil
}

func logLevel(value string) (logkit.Level, error) {
	switch value {
	case "debug":
		return logkit.DebugLevel, nil
	case "info":
		return logkit.InfoLevel, nil
	case "warn":
		return logkit.WarnLevel, nil
	case "error":
		return logkit.ErrorLevel, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", value)
	}
}

func logOutput(value string) (logkit.OutputType, error) {
	switch value {
	case "console":
		return logkit.ConsoleOutput, nil
	case "file":
		return logkit.FileOutput, nil
	case "both":
		return logkit.BothOutput, nil
	default:
		return 0, fmt.Errorf("unknown log output %q", value)
	}
}
