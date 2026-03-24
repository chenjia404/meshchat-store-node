package config

import "testing"

func TestDefaultListenAddrs(t *testing.T) {
	addrs := DefaultListenAddrs(9001)
	if len(addrs) != 2 {
		t.Fatalf("len(addrs) = %d, want 2", len(addrs))
	}
	if addrs[0] != "/ip4/0.0.0.0/tcp/9001" {
		t.Fatalf("addrs[0] = %q", addrs[0])
	}
	if addrs[1] != "/ip4/0.0.0.0/udp/9001/quic-v1" {
		t.Fatalf("addrs[1] = %q", addrs[1])
	}
}

func TestBuildAnnounceAddrs(t *testing.T) {
	listenAddrs := []string{
		"/ip4/0.0.0.0/tcp/9001",
		"/ip4/127.0.0.1/udp/9001/quic-v1",
	}

	addrs, err := BuildAnnounceAddrs(listenAddrs, "203.0.113.10")
	if err != nil {
		t.Fatalf("BuildAnnounceAddrs() error = %v", err)
	}
	if len(addrs) != 2 {
		t.Fatalf("len(addrs) = %d, want 2", len(addrs))
	}
	if addrs[0] != "/ip4/203.0.113.10/tcp/9001" {
		t.Fatalf("addrs[0] = %q", addrs[0])
	}
	if addrs[1] != "/ip4/203.0.113.10/udp/9001/quic-v1" {
		t.Fatalf("addrs[1] = %q", addrs[1])
	}
}

func TestBuildAnnounceAddrsInvalidIP(t *testing.T) {
	if _, err := BuildAnnounceAddrs(DefaultListenAddrs(9001), "not-an-ip"); err == nil {
		t.Fatal("BuildAnnounceAddrs() error = nil, want error")
	}
}
