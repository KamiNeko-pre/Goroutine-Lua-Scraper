package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
)

func startPprofServer(port int) {
	if port <= 0 {
		return
	}
	go func() { log.Printf("pprof: %v", http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)) }()
}
