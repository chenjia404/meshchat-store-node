package netutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var defaultPublicIPEndpoints = []string{
	"https://api.ipify.org",
	"https://checkip.amazonaws.com",
	"https://icanhazip.com",
}

func DetectPublicIP(ctx context.Context, client *http.Client, endpoints []string) (string, error) {
	if client == nil {
		client = &http.Client{
			Timeout: 3 * time.Second,
		}
	}
	if len(endpoints) == 0 {
		endpoints = defaultPublicIPEndpoints
	}

	var errs []string
	for _, endpoint := range endpoints {
		ip, err := detectPublicIPFromEndpoint(ctx, client, endpoint)
		if err == nil {
			return ip, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", endpoint, err))
	}

	return "", fmt.Errorf("detect public ip failed: %s", strings.Join(errs, "; "))
}

func detectPublicIPFromEndpoint(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("invalid ip response: %q", ip)
	}

	if ipv4 := parsedIP.To4(); ipv4 != nil {
		return ipv4.String(), nil
	}
	return parsedIP.String(), nil
}
