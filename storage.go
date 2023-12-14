package main

import (
	"fmt"
	"time"
)

type Sample struct {
	Address string
	At      time.Time
	RttMs   uint64
}

func (s Sample) String() string {
	if s.RttMs > 0 {
		return fmt.Sprintf("%s %s RTT=%d ms", s.At.Format("2006-01-02T15:04:05"), s.Address, s.RttMs)
	} else {
		return fmt.Sprintf("%s %s TIMEOUT", s.At.Format("2006-01-02T15:04:05"), s.Address)
	}
}

func (s Sample) IsOnline() bool {
	return s.RttMs > 0
}

type Event struct {
	Address  string
	At       time.Time
	Duration time.Duration
	Online   bool
}

func (e Event) String() string {
	return fmt.Sprintf("%s: %s is %s for %d ms", e.At.Format("2006-01-02T15:04:05"), e.Address, b2s[e.Online], e.DurMs())
}

func (e Event) DurMs() uint64 {
	return uint64(e.Duration / time.Millisecond)
}

type Adder interface {
	Add(sample Sample) error
}

type Storage interface {
	Close() error
	ListAddresses() ([]string, error)
	Filter(address string, from time.Time, to time.Time) ([]Sample, error)
	Prune(address string, to time.Time) error
}
