package chat

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	addr        string
	metricsAddr string
	logger      *slog.Logger
	reg         *Registry
	listener    net.Listener
}

func NewServer(addr, metricsAddr string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		addr:        addr,
		metricsAddr: metricsAddr,
		logger:      logger,
		reg:         NewRegistry(128, logger),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln

	go s.reg.Run()
	go s.acceptLoop(ln)
	go s.serveMetrics()

	s.logger.Info("server started", "addr", s.addr)
	return nil
}

func (s *Server) serveMetrics() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	s.logger.Info("metrics available", "addr", s.metricsAddr+"/metrics")
	if err := http.ListenAndServe(s.metricsAddr, mux); err != nil {
		s.logger.Error("metrics server error", "error", err)
	}
}

func (s *Server) Stop() {
	s.logger.Info("shutting down")

	if s.listener != nil {
		s.listener.Close()
	}

	s.reg.Stop()
	s.reg.Wait()

	s.logger.Info("shutdown complete")
}

func (s *Server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			// listener가 닫히면 여기로 옴 — 정상 종료
			return
		}

		s.logger.Info("client connected", "addr", conn.RemoteAddr().String())

		c := &Client{
			Conn: conn,
			Out:  make(chan string, 32),
		}
		go HandleSession(c, s.reg.Events())
	}
}
