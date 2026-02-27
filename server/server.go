package server

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/cryptowizard0/vmdocker_agent/common"
	"github.com/cryptowizard0/vmdocker_agent/runtime"
	"github.com/gin-gonic/gin"
)

var log = common.NewLog("server")

type Server struct {
	engine *gin.Engine
	port   int
	srv    *http.Server

	runtime *runtime.Runtime
	aoPath  string
	// outgoingChan chan nodeSchema.Outgoing // used to send messages to Cu
}

func New(port int) *Server {
	engine := gin.Default()
	return &Server{
		engine: engine,
		port:   port,
		// outgoingChan: make(chan nodeSchema.Outgoing),
		aoPath: getEnvOrDefault("AO_PATH", "./ao/2.0.1"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (s *Server) Run() error {
	log.Info("server running", "port", s.port)

	// create context
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// start api
	endpoint := fmt.Sprintf(":%d", s.port)
	go s.runAPI(endpoint)

	// handle message from channel, just print it
	// go func() {
	// 	for {
	// 		select {
	// 		case msg := <-s.outgoingChan:
	// 			log.Info("received message", "msg", msg)
	// 		case <-ctx.Done():
	// 			return
	// 		}
	// 	}
	// }()

	// wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// close channel
	// close(s.outgoingChan)

	return s.closeAPI()
}

func (s *Server) Close() error {
	return s.closeAPI()
}
