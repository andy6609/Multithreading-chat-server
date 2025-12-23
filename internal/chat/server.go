package chat

import (
	"log"
	"net"
)

type Server struct {
	addr   string
	logger *log.Logger
	reg    *Registry
}

func NewServer(addr string, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		addr:   addr,
		logger: logger,
		reg:    NewRegistry(128),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	go s.reg.Run()
	go s.acceptLoop(ln)
	return nil
}

func (s *Server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			s.logger.Printf("accept error: %v", err)
			return
		}

		c := &Client{
			Conn: conn,
			Out:  make(chan string, 32),
		}
		go HandleSession(c, s.reg.Events())
	}
}


