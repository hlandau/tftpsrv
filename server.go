package tftpsrv

import (
	"bytes"
	"encoding/binary"
	"net"
	"time"

	denet "github.com/hlandau/degoutils/net"
)

const (
	opReadRequest  uint16 = 1
	opWriteRequest        = 2
	opData                = 3
	opAck                 = 4
	opError               = 5
	opOptAck              = 6
)

// TFTP server.
type Server struct {
	Addr                  string                   // TCP address to listen on, ":tftp" if empty
	ReadHandler           func(req *Request) error // Handler for read requests
	RetransmissionTimeout time.Duration            // Time to wait before retransmitting (default 1s)
	RequestTimeout        time.Duration            // Time to wait before timing out connection (default 4*RetransmissionTimeout)

	socket   *net.UDPConn
	requests map[string]*Request
}

// Listens at the specified address (default ":tftp").
func (s *Server) Listen() error {
	if s.socket != nil {
		return nil
	}

	if s.Addr == "" {
		s.Addr = ":tftp"
	}

	addr, err := net.ResolveUDPAddr("udp", s.Addr)
	if err != nil {
		return err
	}

	sock, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	s.socket = sock

	return nil
}

// Starts processing TFTP requests. Does not return unless an error occurs.
// Calls Listen() automatically if it has not already been called. You may call
// Listen() separately if you need to drop privileges.
func (s *Server) ListenAndServe() error {
	err := s.Listen()
	if err != nil {
		return err
	}

	s.requests = make(map[string]*Request)

	if s.RetransmissionTimeout == 0 {
		s.RetransmissionTimeout = 1 * time.Second
	}

	if s.RequestTimeout == 0 {
		s.RequestTimeout = 4 * s.RetransmissionTimeout
	}

	return s.loop()
}

func (s *Server) loop() error {
	for {
		buf, addr, err := denet.ReadDatagramFromUDP(s.socket)
		if err != nil {
			// XXX: error handling
			continue
		}

		// We have a reasonable datagram.
		// TODO: IPv6 scoped addressing zones.
		s.handleDatagram(buf, addr)
	}
	//return nil
}

func (s *Server) handleDatagram(buf []byte, addr *net.UDPAddr) error {
	if len(buf) < 4 {
		// undersize datagram, ignore
		return nil
	}

	var opcode uint16
	br := bytes.NewBuffer(buf)
	err := binary.Read(br, binary.BigEndian, &opcode)
	if err != nil {
		return err
	}

	switch opcode {
	case opReadRequest:
		return s.handleReadRequest(br, addr)
	case opAck:
		return s.handleAck(br, addr)
	default:
		return s.handleUnknownOpcode(addr)
	}
}

func (s *Server) handleReadRequest(br *bytes.Buffer, addr *net.UDPAddr) error {
	// Request body starts with zero-terminated filename.
	filename_b, err := br.ReadBytes(0)
	if err != nil {
		return err
	}
	filename := cstrToString(filename_b)

	// Filename is followed by a zero-terminated mode string.
	mode_b, err := br.ReadBytes(0)
	if err != nil {
		return err
	}
	mode := cstrToString(mode_b)

	req := &Request{
		addr:       addr,
		ackChannel: make(chan uint16, 8),
		Filename:   filename,
		Mode:       mode,
		server:     s,
		blockSize:  512,
		blockNum:   1,
	}

	// Option handling
	for {
		// Get zero-terminated option name.
		opt_k_b, err := br.ReadBytes(0)
		if err != nil {
			// No more options.
			break
		}
		opt_k := cstrToString(opt_k_b)

		// Get zero-terminated option value.
		opt_v_b, err := br.ReadBytes(0)
		if err != nil {
			// If there is a key, there must be a value.
			return err
		}
		opt_v := cstrToString(opt_v_b)

		// Set option. Ignore errors.
		req.setOption(opt_k, opt_v)
	}

	// Register request
	// XXX: Hacky request tracking structure.
	s.requests[req.name()] = req

	go req.loop()

	return nil
}

func (s *Server) handleAck(br *bytes.Buffer, addr *net.UDPAddr) error {
	var bnum uint16
	err := binary.Read(br, binary.BigEndian, &bnum)
	if err != nil {
		return err
	}

	tx, ok := s.requests[nameFromAddr(addr)]
	if ok {
		// Send acknowledgement number to the request processor.
		// If the ack channel is full for whatever reason, discard.
		select {
		case tx.ackChannel <- bnum:
		default:
		}

		return nil
	}

	// No valid request, send an ERROR.
	return s.sendTftpErrorPacket(addr, ErrUnknownTransfer, "Unknown transfer")
}

func (s *Server) handleUnknownOpcode(addr *net.UDPAddr) error {
	return s.sendTftpErrorPacket(addr, ErrIllegalOpcode, "Unknown opcode")
}
