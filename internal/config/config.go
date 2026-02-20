package config

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        int
	BindAddr    string
	CardIndex   uint
	LogLevel    string
	MonitorFile string
}

func Load() (*Config, error) {

	cfg := &Config{Port: 8080, BindAddr: "0.0.0.0", CardIndex: 0, LogLevel: "info", MonitorFile: "/etc/asound.conf"}

	if v := os.Getenv("ALSAMIXER_WEB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		} else {
			return nil, fmt.Errorf("invalid ALSAMIXER_WEB_PORT: %q", v)
		}
	}
	if v := os.Getenv("ALSAMIXER_WEB_BIND"); v != "" {
		cfg.BindAddr = v
	}
	if v := os.Getenv("ALSAMIXER_WEB_CARD"); v != "" {
		if c, err := strconv.ParseUint(v, 10, 64); err == nil {
			cfg.CardIndex = uint(c)
		} else {
			return nil, fmt.Errorf("invalid ALSAMIXER_WEB_CARD: %q", v)
		}
	}
	if v := os.Getenv("ALSAMIXER_WEB_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("ALSAMIXER_WEB_MONITOR_FILE"); v != "" {
		cfg.MonitorFile = v
	}

	fs := flag.NewFlagSet("alsamixer-web", flag.ContinueOnError)
	var portFlag int
	var bindFlag string
	var cardFlag uint
	var logLevelFlag string
	var monitorFileFlag string
	fs.IntVar(&portFlag, "port", cfg.Port, "Server port")
	fs.IntVar(&portFlag, "p", cfg.Port, "Server port (shorthand)")
	fs.StringVar(&bindFlag, "bind", cfg.BindAddr, "Bind address")
	fs.StringVar(&bindFlag, "b", cfg.BindAddr, "Bind address (shorthand)")
	fs.UintVar(&cardFlag, "card", cfg.CardIndex, "ALSA card index")
	fs.UintVar(&cardFlag, "c", cfg.CardIndex, "ALSA card index (shorthand)")
	fs.StringVar(&logLevelFlag, "log-level", cfg.LogLevel, "Log level")
	fs.StringVar(&monitorFileFlag, "monitor-file", cfg.MonitorFile, "Path to ALSA config file to monitor")
	var helpFlag bool
	fs.BoolVar(&helpFlag, "help", false, "Show help")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}
	if helpFlag {
	}
	cfg.Port = portFlag
	cfg.BindAddr = bindFlag
	cfg.CardIndex = cardFlag
	if logLevelFlag != "" {
		cfg.LogLevel = logLevelFlag
	}
	if monitorFileFlag != "" {
		cfg.MonitorFile = monitorFileFlag
	}
	return cfg, nil
}

func HelpText() string {
	var buf bytes.Buffer
	fs := flag.NewFlagSet("alsamixer-web", flag.ContinueOnError)
	fs.Int("port", 8080, "Server port")
	fs.Int("p", 8080, "Server port (shorthand)")
	fs.String("bind", "0.0.0.0", "Bind address")
	fs.String("b", "0.0.0.0", "Bind address (shorthand)")
	fs.Uint("card", 0, "ALSA card index")
	fs.Uint("c", 0, "ALSA card index (shorthand)")
	fs.String("log-level", "info", "Log level")
	fs.String("monitor-file", "/etc/asound.conf", "Path to ALSA config file to monitor")
	fs.SetOutput(&buf)
	fs.Usage()
	return buf.String()
}
