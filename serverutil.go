package tftpsrv

import "net"
import "encoding/binary"
import "bytes"

/* sendTftpDataPacket
 * ------------------
 */
func (s *Server) sendTftpDataPacket(
	addr *net.UDPAddr, blockNum uint16, buf []byte) error {
	u := opData
	b := &bytes.Buffer{}

	err := binary.Write(b, binary.BigEndian, &u)
	if err != nil {
		return err
	}

	err = binary.Write(b, binary.BigEndian, &blockNum)
	if err != nil {
		return err
	}

	_, err = b.Write(buf)
	if err != nil {
		return err
	}

	_, err = s.socket.WriteToUDP(b.Bytes(), addr)
	if err != nil {
		return err
	}

	return nil
}

// TFTP protocol error code.
type Error uint16

const (
	ErrGeneric         Error = 0
	ErrFileNotFound          = 1
	ErrAccessViolation       = 2
	ErrDiskFull              = 3
	ErrIllegalOpcode         = 4
	ErrUnknownTransfer       = 5
	ErrAlreadyExists         = 6
	ErrUnknownUser           = 7
	ErrOptNegFail            = 8
)

func (s *Server) sendTftpErrorPacket(
	addr *net.UDPAddr, num Error, msg string) error {
	ec := opError
	bw := new(bytes.Buffer)

	err := binary.Write(bw, binary.BigEndian, &ec)
	if err != nil {
		return err
	}

	err = binary.Write(bw, binary.BigEndian, uint16(num))
	if err != nil {
		return err
	}

	_, err = bw.WriteString(msg)
	if err != nil {
		return err
	}

	bw.Write([]byte{0})

	_, err = s.socket.WriteToUDP(bw.Bytes(), addr)
	return err
}

func (s *Server) sendTftpOptNegPacket(addr *net.UDPAddr, options map[string]string) error {
	ec := opOptAck
	bw := new(bytes.Buffer)

	err := binary.Write(bw, binary.BigEndian, &ec)
	if err != nil {
		return err
	}

	for k, v := range options {
		bw.WriteString(k)
		bw.Write([]byte{0})
		bw.WriteString(v)
		bw.Write([]byte{0})
	}

	_, err = s.socket.WriteToUDP(bw.Bytes(), addr)
	return err
}

// © 2014 Hugo Landau <hlandau@devever.net>    GPLv3 or later
