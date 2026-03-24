package metrics

import "sync/atomic"

type Metrics struct {
	StoreRequestsTotal         atomic.Uint64
	StoreSuccessTotal          atomic.Uint64
	StoreDuplicateTotal        atomic.Uint64
	FetchRequestsTotal         atomic.Uint64
	FetchMessagesReturnedTotal atomic.Uint64
	AckRequestsTotal           atomic.Uint64
	AckDeletedTotal            atomic.Uint64
	GCDeletedTotal             atomic.Uint64
	InvalidSignatureTotal      atomic.Uint64
	RateLimitedTotal           atomic.Uint64
}

func New() *Metrics {
	return &Metrics{}
}
