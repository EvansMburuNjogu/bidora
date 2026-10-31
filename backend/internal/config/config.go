// Package config validates the deliberately local-only health smoke profile.
package config

import (
	"errors"
	"net"
	"strconv"
)

type Config struct{ Address string }

func Load(getenv func(string) string) (Config, error) {
	if getenv("BIDORA_PROFILE") != "health-only" {
		return Config{}, errors.New("BIDORA_PROFILE must be explicitly set to health-only")
	}
	if env := getenv("BIDORA_ENV"); env != "local" && env != "test" {
		return Config{}, errors.New("BIDORA_ENV must be local or test; production is not supported")
	}
	addr := getenv("BIDORA_HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return Config{}, errors.New("BIDORA_HTTP_ADDR must be a loopback IP and port")
	}
	ip := net.ParseIP(host)
	n, err := strconv.Atoi(port)
	if ip == nil || !ip.IsLoopback() || err != nil || n < 1 || n > 65535 {
		return Config{}, errors.New("BIDORA_HTTP_ADDR must be a loopback IP and port from 1 to 65535")
	}
	return Config{Address: addr}, nil
}
