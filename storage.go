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
	return fmt.Sprintf("host %s was %s from %s for %.1f s", e.Address, b2s[e.Online], e.At.Format("2006-01-02T15:04:05"), e.DurF64())
}

func (e Event) DurMs() uint64 {
	return uint64(e.Duration / time.Millisecond)
}

func (e Event) DurF64() float64 {
	return float64(e.Duration.Microseconds() / 1000000)
}

type Adder interface {
	Add(sample Sample) error
}

type Storage interface {
	Close() error
	ListAddresses() ([]string, error)
	Filter(address string, from time.Time, to time.Time) ([]Sample, error)
	Prune(address string, to time.Time) error
	Add(sample Sample) error
	EventRegister(event Event) error
	EventOpen(event Event) error
	EventUpdate(event Event) error
	EventClose(event Event) error
}
