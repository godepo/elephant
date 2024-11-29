package cluster

import "sync/atomic"

type roundRobin struct {
	next *atomic.Int64
}

func DefaultLoadBalancer() LoadBalancer {
	rr := &roundRobin{
		next: &atomic.Int64{},
	}
	return rr.Balance
}

func (r *roundRobin) Balance(fellows []Pool) Pool {
	if len(fellows) == 0 {
		return nil
	}
	ix := int(r.next.Add(1)) % len(fellows)
	return fellows[ix]
}
