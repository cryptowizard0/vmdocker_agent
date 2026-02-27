package main

import (
	"github.com/cryptowizard0/vmdocker_agent/server"
)

func main() {
	server := server.New(8080)
	server.Run()
}
