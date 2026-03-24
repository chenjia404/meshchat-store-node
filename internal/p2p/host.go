package p2p

import (
	"github.com/libp2p/go-libp2p"
	corehost "github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
)

func NewHost(listenAddrs []string) (corehost.Host, error) {
	addrs := make([]multiaddr.Multiaddr, 0, len(listenAddrs))
	for _, addr := range listenAddrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, ma)
	}
	return libp2p.New(libp2p.ListenAddrs(addrs...))
}
