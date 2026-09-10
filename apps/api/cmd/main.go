// Package main 是 iot-pillot 后端 API 服务的入口。
package main

import (
	"log"
)

func main() {
	srv, teardown, err := InitializeApp()
	defer teardown()
	if err != nil {
		log.Fatalf("initialize app: %v", err)
	}
	if err := srv.Run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
