package main

import (
	"log"
	"time"

	"github.com/zekroTJA/timedmap/v2"
)

func main() {
	tm := timedmap.New(5 * time.Second)
	tm.Set("hey", "ho", 3*time.Second, expiringCallback)
	tm.Set("whats", "up", 5*time.Second-100*time.Millisecond, expiringCallback)

	for i := 0; i < 6; i++ {
		printkv(tm, "hey")
		printkv(tm, "whats")
		time.Sleep(2 * time.Second)
	}
}

func printkv(tm timedmap.Map, key interface{}) {
	log.Printf("%5s - %+v", key, tm.GetValue(key))
}

func expiringCallback(v interface{}) {
	log.Printf("%+v expired", v)
}
