package chat

import (
	"log/slog"
	"net"
)

type Server struct {
	addr     string
	logger   *slog.Logger
	reg      *Registry
	listener net.Listener
}

func NewServer(addr string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		addr:   addr,
		logger: logger,
		reg:    NewRegistry(128, logger),
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

	s.logger.Info("server started", "addr", s.addr)
	return nil
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
