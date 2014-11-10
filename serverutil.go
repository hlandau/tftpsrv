package tftpsrv
import "net"
import "encoding/binary"
import "bytes"

/* sendTftpDataPacket
 * ------------------
 */
func (self *Server) sendTftpDataPacket(
    addr *net.UDPAddr, blockNum uint16, buf []byte) error {
  var u uint16 = OP_DATA
  b   := new(bytes.Buffer)

  err := binary.Write(b, binary.BigEndian, &u)
  if err != nil {
    return err
  }

  err  = binary.Write(b, binary.BigEndian, &blockNum)
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

/* sendTftpErrorPacket
 * -------------------
 */
const ERR_GENERIC          = 0
const ERR_FILE_NOT_FOUND   = 1
const ERR_ACCESS_VIOLATION = 2
const ERR_DISK_FULL        = 3
const ERR_ILLEGAL_OPCODE   = 4
const ERR_UNKNOWN_TRANSFER = 5
const ERR_ALREADY_EXISTS   = 6
const ERR_UNKNOWN_USER     = 7
const ERR_OPTNEG_FAIL      = 8

func (self *Server) sendTftpErrorPacket(
  addr *net.UDPAddr, num uint16, msg string) error {
  var ec uint16 = OP_ERROR
  bw := new(bytes.Buffer)

  err := binary.Write(bw, binary.BigEndian, &ec)
  if err != nil {
    return err
  }

  err  = binary.Write(bw, binary.BigEndian, num)
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
