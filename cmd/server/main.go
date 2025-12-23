package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/andy6609/Multithreading-chat-server/internal/chat"
)

func main() {
	addr := flag.String("addr", ":5000", "listen address")
	flag.Parse()

	logger := log.New(os.Stdout, "[chat-server] ", log.LstdFlags|log.Lmicroseconds)

	srv := chat.NewServer(*addr, logger)
	if err := srv.Start(); err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
	logger.Printf("listening on %s", *addr)

	// MVP: keep running until SIGINT/SIGTERM. (Graceful shutdown is out-of-scope.)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	logger.Printf("signal received, exiting")
}


