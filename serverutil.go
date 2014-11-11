package tftpsrv

import "net"
import "encoding/binary"
import "bytes"

/* sendTftpDataPacket
 * ------------------
 */
func (self *Server) sendTftpDataPacket(
	addr *net.UDPAddr, blockNum uint16, buf []byte) error {
	var u uint16 = opData
	b := new(bytes.Buffer)

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

	_, err = self.socket.WriteToUDP(b.Bytes(), addr)
	if err != nil {
		return err
	}

	return nil
}

// TFTP protocol error codes.
type Error uint16

const (
	ErrGeneric Error   = 0
	ErrFileNotFound    = 1
	ErrAccessViolation = 2
	ErrDiskFull        = 3
	ErrIllegalOpcode   = 4
	ErrUnknownTransfer = 5
	ErrAlreadyExists   = 6
	ErrUnknownUser     = 7
	ErrOptNegFail      = 8
	)

func (self *Server) sendTftpErrorPacket(
	addr *net.UDPAddr, num Error, msg string) error {
	var ec uint16 = opError
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

	_, err = self.socket.WriteToUDP(bw.Bytes(), addr)
	return err
}
