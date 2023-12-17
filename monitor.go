package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

var ResurrectInterval = 3 * ReaperInterval
var NullTime time.Time = time.Time{}

type Watcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	storage Storage
	samples chan Sample
	mutex   sync.Mutex
	wg      sync.WaitGroup
	probers map[string]context.CancelFunc
	events  map[string]Event
}

func (watcher *Watcher) process(sample Sample) {
	if !sample.At.After(NullTime) || sample.Address == "" {
		panic(fmt.Sprintf("bad sample %#v", sample))
	}
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()
	evt, ok := watcher.events[sample.Address]
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
		watcher.events[sample.Address] = evt
		if err := watcher.storage.EventOpen(evt); err != nil {
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
				watcher.events[sample.Address] = evt
				if err := watcher.storage.EventUpdate(evt); err != nil {
					log.Printf("EventUpdate(%v) error: %s", evt, err.Error())
				}
				return
			} else {
				// switch to the new state
				// log.Printf("A SWITCHER: %s", sample)
				oldevt := evt.String()
				if err := watcher.storage.EventClose(evt); err != nil {
					log.Printf("EventClose(%v) error: %s", evt, err.Error())
				}
				// start the new event
				evt = Event{
					Address:  sample.Address,
					At:       sample.At,
					Duration: 0,
					Online:   sample.IsOnline(),
				}
				if err := watcher.storage.EventOpen(evt); err != nil {
					log.Printf("EventOpen(%v) error: %s", evt, err.Error())
				}
				watcher.events[sample.Address] = evt
				log.Printf("%s; %s", oldevt, b2s2[sample.IsOnline()])
				return
			}
			// panic("unreachable point")
		}
		// panic("unreachable point")
	}
	// panic("unreachable point")
}

func NewWatcher(ctx context.Context, dbpath string) *Watcher {
	stg, err := NewSqliteStorage(dbpath)
	if err != nil {
		panic(err)
	}
	basectx, cancel := context.WithCancel(ctx)
	watcher := &Watcher{
		ctx:     basectx,
		cancel:  cancel,
		storage: stg,
		samples: make(chan Sample, 100),
		mutex:   sync.Mutex{},
		wg:      sync.WaitGroup{},
		probers: map[string]context.CancelFunc{},
		events:  map[string]Event{},
	}

	go func() {
		defer watcher.storage.Close()
		log.Printf("watch loop started")
		for {
			select {
			case <-watcher.ctx.Done():
				log.Printf("context closed. wait for all subprocesses exited")
				watcher.wg.Wait()
				log.Printf("all subprocesses exited")
				return
			case sample := <-watcher.samples:
				// log.Println(sample.String())
				if err := watcher.storage.Add(sample); err != nil {
					log.Printf("Add(%v) error: %s", sample, err.Error())
				}
				watcher.process(sample)
			}
		}
	}()
	return watcher
}

func (watcher *Watcher) LogStatus() {
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()
	log.Println("--- status ---")
	for _, event := range watcher.events {
		log.Println(event.String())
	}
	log.Println("---- end ----")
}

func (watcher *Watcher) AddHost(host string) {
	if _, ok := watcher.probers[host]; ok {
		log.Printf("host %s is already under monitoring", host)
		return
	}
	ctx, cancel := context.WithCancel(watcher.ctx)
	watcher.wg.Add(1)
	go func() {
		defer watcher.wg.Done()
		for {
			if err := monitor(ctx, host, watcher.samples); err != nil {
				log.Printf("monitoring for %q failed with error: %s", host, err.Error())
				time.Sleep(ResurrectInterval)
				log.Printf("resurrect monitoring for %q", host)
			} else {
				watcher.mutex.Lock()
				delete(watcher.probers, host)
				watcher.mutex.Unlock()
				log.Printf("monitor for %q finished", host)
				return
			}
		}
	}()
	watcher.probers[host] = cancel
	log.Printf("added host %s", host)
}

func (watcher *Watcher) RemoveHost(host string) {
	if cancel, ok := watcher.probers[host]; ok {
		log.Printf("remove host %s", host)
		cancel()
	} else {
		log.Printf("host %s is not found", host)
		return
	}
}
