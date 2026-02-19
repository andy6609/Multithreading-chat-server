package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/andy6609/Multithreading-chat-server/internal/chat"
)

func main() {
	addr := flag.String("addr", ":5000", "chat listen address")
	metricsAddr := flag.String("metrics-addr", ":9090", "metrics listen address")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	srv := chat.NewServer(*addr, *metricsAddr, logger)
	if err := srv.Start(); err != nil {
		logger.Error("failed to start server", "error", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	srv.Stop()
}
