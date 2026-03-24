package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Node struct {
		ListenAddrs     []string `yaml:"listen_addrs"`
		AnnounceAddrs   []string `yaml:"announce_addrs"`
		IdentityKeyPath string   `yaml:"identity_key_path"`
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
	cfg.Node.ListenAddrs = DefaultListenAddrs(4001)
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

func DefaultListenAddrs(port int) []string {
	return []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", port),
	}
}

func BuildAnnounceAddrs(listenAddrs []string, announceIP string) ([]string, error) {
	parsedIP := net.ParseIP(announceIP)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid announce ip: %q", announceIP)
	}

	protocol := "ip6"
	normalizedIP := parsedIP.String()
	if ipv4 := parsedIP.To4(); ipv4 != nil {
		protocol = "ip4"
		normalizedIP = ipv4.String()
	}

	addrs := make([]string, 0, len(listenAddrs))
	for _, addr := range listenAddrs {
		parts := strings.Split(addr, "/")
		if len(parts) < 4 {
			return nil, fmt.Errorf("unsupported listen addr: %q", addr)
		}
		if parts[1] != "ip4" && parts[1] != "ip6" {
			return nil, fmt.Errorf("unsupported listen addr protocol: %q", addr)
		}
		parts[1] = protocol
		parts[2] = normalizedIP
		addrs = append(addrs, strings.Join(parts, "/"))
	}
	return addrs, nil
}
