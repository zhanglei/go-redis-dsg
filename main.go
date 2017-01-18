//Distributed message consumer and generator for Redis.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/ekhabarov/bbeye"
)

func main() {
	runtime.GOMAXPROCS(1)
	//runtime.GOMAXPROCS(runtime.NumCPU())

	ge := flag.Bool("getErrors", false, "Get all errors from Redis")
	flag.Parse()

	cfg := ReadConfig()
	done := make(chan struct{})

	redisPool := NewRedisPool(cfg.redis.url, cfg.redis.poolSize)

	if *ge {
		e := NewErrorReader(redisPool, cfg.redis.errQueue)
		if e.Count() < 1 {
			log.Println("Errors list is empty.")
			return
		}
		for _, m := range e.ReadErrors() {
			fmt.Println(m)
		}
		return
	}

	c := NewConsumer(redisPool, cfg.redis.queue, cfg.redis.errQueue, cfg.consumer.maxGoroutines)
	g := NewGen(redisPool, cfg.redis.queue, cfg.generator.interval, cfg.generator.pingInterval)

	var processed chan ProcessedMessage

	if g.AcquireLock() {
		go g.Start()
	} else {
		processed = c.Start()
	}

	//go StartPing(g, c)
	go bbeye.Run("127.0.0.1:" + os.Getenv("MPORT"))

	//Exit timeout
	go func(d chan<- struct{}) {
		time.Sleep(time.Second * 300)
		d <- struct{}{}
	}(done)

	//go func(g *Generator) {
	//for {
	//select {
	//case <-time.After(time.Second * 5):
	//if g.IsActive() {
	//log.Println("Generator off")
	//g.Stop()
	//} else {
	//log.Println("Generator on")
	//go g.Start()
	//}
	//}
	//}
	//}(g)

	go func(c *Consumer) {
		for {
			select {
			case <-time.After(time.Second * 5):
				if c.IsActive() {
					log.Println("Consumer off.")
					c.Stop()
				} else {
					log.Println("Consumer on")
					processed = c.Start()
				}
			}
		}
	}(c)

	for {
		select {
		case <-processed:
		case <-done:
			log.Println("Done.")
			return

		}
	}
}
