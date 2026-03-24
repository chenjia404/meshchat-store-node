package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Node struct {
		ListenAddrs []string `yaml:"listen_addrs"`
	} `yaml:"node"`
	Store struct {
		DataDir                 string `yaml:"data_dir"`
		DefaultTTLSec           int64  `yaml:"default_ttl_sec"`
		MaxTTLSec               int64  `yaml:"max_ttl_sec"`
		MaxMessageSize          int    `yaml:"max_message_size"`
		MaxMessagesPerRecipient int    `yaml:"max_messages_per_recipient"`
		MaxBytesPerRecipient    int64  `yaml:"max_bytes_per_recipient"`
		FetchLimitMax           int    `yaml:"fetch_limit_max"`
	} `yaml:"store"`
	GC struct {
		IntervalSec int `yaml:"interval_sec"`
		BatchSize   int `yaml:"batch_size"`
	} `yaml:"gc"`
	RateLimit struct {
		PerSenderPerMinute int `yaml:"per_sender_per_minute"`
	} `yaml:"rate_limit"`
}

func Default() Config {
	var cfg Config
	cfg.Node.ListenAddrs = []string{
		"/ip4/0.0.0.0/tcp/4001",
		"/ip4/0.0.0.0/udp/4001/quic-v1",
	}
	cfg.Store.DataDir = "./data"
	cfg.Store.DefaultTTLSec = 2592000
	cfg.Store.MaxTTLSec = 2592000
	cfg.Store.MaxMessageSize = 262144
	cfg.Store.MaxMessagesPerRecipient = 5000
	cfg.Store.MaxBytesPerRecipient = 134217728
	cfg.Store.FetchLimitMax = 500
	cfg.GC.IntervalSec = 10
	cfg.GC.BatchSize = 1000
	cfg.RateLimit.PerSenderPerMinute = 100
	return cfg
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
