package main

import (
	"context"
	"math/rand"
	"sync"
	"time"

	ping "github.com/prometheus-community/pro-bing"
)

var (
	Fake           = false
	PingRaw        = true
	PingInterval   = 1 * time.Second
	PingDrift      = 100 * time.Millisecond
	PingTimeout    = 3 * time.Second
	ReaperInterval = PingTimeout * 3
	// StatInterval      = 5 * time.Minute
)

func monitor(ctx context.Context, addr string, ch chan<- Sample) error {
	pinger, err := ping.NewPinger(addr)
	if err != nil {
		return err
	}
	subctx, cancel := context.WithCancel(ctx)
	defer cancel()
	push := func(at time.Time, rtt time.Duration, ok bool) {
		var ms uint64 = 0
		if ok {
			ms = uint64(rtt / time.Millisecond)
			// ms = uint64(time.Since(at) / time.Millisecond)
			if ms == 0 {
				ms = 1
			}
		}
		sample := Sample{
			Address: addr,
			At:      at,
			RttMs:   ms,
		}
		select {
		case ch <- sample:
			// added to channel
		default:
			// overflow
		}
	} // push()

	mx := sync.Mutex{}
	active := map[int]time.Time{}
	pinger.SetPrivileged(PingRaw)
	pinger.RecordRtts = false
	pinger.Count = -1
	pinger.Interval = PingInterval + time.Duration(rand.Int63()%int64(PingDrift))
	pinger.OnSend = func(p *ping.Packet) {
		// log.Printf("OnSend(%#v)", p)
		if Fake {
			return
		}
		mx.Lock()
		defer mx.Unlock()
		id := p.Seq

		if t, ok := active[id]; ok {
			// log.Printf("duplicate after %d", time.Since(t))
			push(t, 0, false)
		}
		active[id] = time.Now()
	} // OnSend

	pinger.OnRecv = func(p *ping.Packet) {
		// log.Printf("OnRecv(%#v)", p)
		if Fake {
			return
		}
		mx.Lock()
		defer mx.Unlock()
		id := p.Seq
		t, ok := active[id]
		if !ok {
			// log.Printf("got deadman")
			return // skip duplicates and obsoletes
		}
		push(t, p.Rtt, true)
		delete(active, id)
	} // OnRecv()

	go func() {
		t := time.NewTicker(ReaperInterval)
		// s := time.NewTicker(StatInterval)
		defer t.Stop()
		// defer s.Stop()
		for {
			select {
			case <-subctx.Done():
				// log.Printf("reaper for %s finished", addr)
				return
			// case <-s.C:
			// 	mx.Lock()
			// 	st := pinger.Statistics()
			// 	log.Printf("FOR %-18s Loss: %.2f, avgRtt: %d ms", addr, st.PacketLoss, int64(st.AvgRtt/time.Millisecond))
			// 	mx.Unlock()
			case <-t.C:
				mx.Lock()
				now := time.Now()
				// log.Printf("reaper checks for %d entries for %s", len(active), addr)
				for id, t := range active {
					dt := now.Sub(t)
					if dt > PingTimeout {
						// log.Printf("[%s/%d] is dead (%d > %d)", addr, id, dt, PingTimeout)
						push(t, 0, false)
						delete(active, id)
					}
				}
				mx.Unlock()
			}
		}
	}() // Reaper
	cherr := make(chan error, 1)
	go func() {
		cherr <- pinger.Run()
		close(cherr)
	}() // BgRunner

	select {
	case <-ctx.Done():
		pinger.Stop()
		return nil
	case err := <-cherr:
		return err
	}
}
