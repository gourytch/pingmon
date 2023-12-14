/*
 * Look at https://github.com/go-ping/ping
 * sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
 * OR
 * setcap cap_net_raw=+ep /path/to/your/compiled/binary
 *
 */

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

const DEFAULT_HOSTLIST = "1.1.1.1 8.8.8.8"

var NullTime time.Time = time.Time{}

type Processor struct {
	storage Storage
	evlock  sync.Mutex
	events  map[string]Event
}

func NewProcessor(storage Storage) *Processor {
	return &Processor{
		storage: storage,
		evlock:  sync.Mutex{},
		events:  map[string]Event{},
	}
}

func (p *Processor) Process(sample Sample) {
	if !sample.At.After(NullTime) || sample.Address == "" {
		panic(fmt.Sprintf("bad sample %#v", sample))
	}
	p.evlock.Lock()
	defer p.evlock.Unlock()
	evt, ok := p.events[sample.Address]
	// log.Printf("sample %+v", sample)
	// log.Printf("event  %+v", evt)

	if !ok {
		// an active event is not found for this address. start the new one
		evt = Event{
			Address:  sample.Address,
			At:       sample.At,
			Duration: 0,
			Online:   sample.IsOnline(),
		}
		p.events[sample.Address] = evt
		if err := p.storage.EventOpen(evt); err != nil {
			log.Printf("EventOpen(%v) error: %s", evt, err.Error())
		}
		log.Printf("%s is %s", sample.Address, b2s[sample.IsOnline()])
		return
	} else {
		// an active event for this address is exist
		dt := sample.At.Sub(evt.At)
		if dt < evt.Duration {
			// an obsolete event. just ignore it
			// log.Printf("AN OBSOLETE SAMPLE: %s", sample)
			return
		} else {
			evt.Duration = dt // update duration for the event
			if sample.IsOnline() == evt.Online {
				// A continue indicator for the active event. Update it but don't save it
				// log.Printf("AN ENLARGER: %s", sample)
				p.events[sample.Address] = evt
				if err := p.storage.EventUpdate(evt); err != nil {
					log.Printf("EventUpdate(%v) error: %s", evt, err.Error())
				}
				return
			} else {
				// switch to the new state
				// log.Printf("A SWITCHER: %s", sample)
				if err := p.storage.EventClose(evt); err != nil {
					log.Printf("EventClose(%v) error: %s", evt, err.Error())
				}
				// start the new event
				p.events[sample.Address] = Event{
					Address:  sample.Address,
					At:       sample.At,
					Duration: 0,
					Online:   sample.IsOnline(),
				}
				log.Printf("%s switched to %s", sample.Address, b2s[sample.IsOnline()])
				return
			}
			// panic("unreachable point")
		}
		// panic("unreachable point")
	}
	// panic("unreachable point")
}

func watch(ctx context.Context, samples <-chan Sample) {

	stg, err := NewSqliteStorage("pingmon.sqlite")
	if err != nil {
		panic(err)
	}
	defer stg.Close()
	processor := NewProcessor(stg)

	log.Printf("watcher started")
	for {
		select {
		case <-ctx.Done():
			log.Printf("watcher finished")
			return
		case sample := <-samples:
			// log.Println(sample.String())
			if err := stg.Add(sample); err != nil {
				log.Printf("Add(%v) error: %s", sample, err.Error())
			}
			processor.Process(sample)
		}
	}
}

func test(ctx context.Context, samples chan<- Sample, addr string) {
	go func() {
		MonitorForever(ctx, addr, samples)
	}()
}

func main() {
	hosts := []string{}
	for _, host := range os.Args[1:] {
		if host != "" {
			hosts = append(hosts, host)
		}
	}
	if len(hosts) == 0 {
		godotenv.Load(".env")
		s := os.Getenv("HOSTLIST")
		if s == "" {
			log.Printf("warning: neither HOSTLIST nor arglist given. I will use the default hostlist")
			s = DEFAULT_HOSTLIST
		}
		for _, host := range strings.Split(s, " ") {
			if host != "" {
				hosts = append(hosts, host)
			}
		}

	}
	if len(hosts) == 0 {
		panic("there is nothing to monitor")
	}
	log.Printf("the next hosts will be monitored: %#v", hosts)

	ctx, cancel := context.WithCancel(context.Background())
	samples := make(chan Sample, 100)
	go watch(ctx, samples)
	for _, host := range hosts {
		test(ctx, samples, host)
	}
	osch := make(chan os.Signal, 1)
	signal.Notify(osch, os.Interrupt)
	<-osch
	log.Println("shutting down...")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("done.")
}
