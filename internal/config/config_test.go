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
