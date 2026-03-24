package p2p

import (
	"github.com/libp2p/go-libp2p"
	corehost "github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
)

func NewHost(listenAddrs, announceAddrs []string) (corehost.Host, error) {
	listenMultiaddrs, err := parseMultiaddrs(listenAddrs)
	if err != nil {
		return nil, err
	}

	options := []libp2p.Option{
		libp2p.ListenAddrs(listenMultiaddrs...),
	}
	if len(announceAddrs) > 0 {
		announceMultiaddrs, err := parseMultiaddrs(announceAddrs)
		if err != nil {
			return nil, err
		}
		options = append(options, libp2p.AddrsFactory(func([]multiaddr.Multiaddr) []multiaddr.Multiaddr {
			return append([]multiaddr.Multiaddr(nil), announceMultiaddrs...)
		}))
	}

	return libp2p.New(options...)
}

func parseMultiaddrs(addrs []string) ([]multiaddr.Multiaddr, error) {
	result := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		result = append(result, ma)
	}
	return result, nil
}
