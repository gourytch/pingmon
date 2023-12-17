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
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
)

const DEFAULT_HOSTLIST = "1.1.1.1 8.8.8.8"

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
	log.Printf("the next hosts will be monitored: %v", hosts)

	ctx, cancel := context.WithCancel(context.Background())
	watcher := NewWatcher(ctx, "pingmon.sqlite")
	for _, host := range hosts {
		watcher.AddHost(host)
	}
	osch := make(chan os.Signal, 1)
	signal.Notify(osch, syscall.SIGINT, syscall.SIGTERM)
	<-osch
	log.Println("shutting down...")
	cancel()
	watcher.wg.Wait()
	watcher.LogStatus()
	log.Println("done.")
}
