package tftpsrv
import "net"
import "bytes"
import "encoding/binary"

const OP_READ_REQUEST  = 1
const OP_WRITE_REQUEST = 2
const OP_DATA          = 3
const OP_ACK           = 4
const OP_ERROR         = 5
const OP_OACK          = 6

/* Server
 * ======
 */
type Server struct {
  handler func(req *Request) error
  socket *net.UDPConn
  maxBlockSize int
  ackTimeout int /* ms */
  requests map[string]*Request
}

func (self *Server) handleReadRequest(br *bytes.Buffer, addr *net.UDPAddr) error {
  filename_b, err := br.ReadBytes(0)
  if err != nil {
    return err
  }
  filename := cstrToString(filename_b)

  mode_b, err := br.ReadBytes(0)
  if err != nil {
    return err
  }
  mode := cstrToString(mode_b)

  req := &Request {
    addr: addr,
    ackChannel: make(chan uint16, 16),
    Filename: filename,
    Mode: mode,
    server: self,
    blockSize: 512,
    blockNum: 1,
  }

  for {
    opt_k_b, err := br.ReadBytes(0)
    if err != nil {
      // No more options.
      break
    }
    opt_k := cstrToString(opt_k_b)

    opt_v_b, err := br.ReadBytes(0)
    if err != nil {
      // If there is a key, there must be a value.
      return err
    }
    opt_v := cstrToString(opt_v_b)

    req.setOption(opt_k, opt_v)
  }

  self.requests[req.name()] = req
  go req.loop(self)

  return nil
}

func (self *Server) handleAck(br *bytes.Buffer, addr *net.UDPAddr) error {
  var bnum uint16
  err := binary.Read(br, binary.BigEndian, &bnum)
  if err != nil {
    return err
  }

  tx, ok := self.requests[nameFromAddr(addr)]
  if ok {
    tx.ackChannel <- bnum
  }

  /* TODO: ACKs for invalid requests are ignored */
  return nil
}

func (self *Server) handleUnknownOpcode(addr *net.UDPAddr) error {
  return self.sendTftpErrorPacket(addr, ERR_ILLEGAL_OPCODE, "Unknown opcode")
}

func (self *Server) handleDatagram(buf []byte, addr *net.UDPAddr) error {
  var opcode uint16
  br  := bytes.NewBuffer(buf)
  err := binary.Read(br, binary.BigEndian, &opcode)
  if err != nil {
    return err
  }

  switch opcode {
    case OP_READ_REQUEST:
      return self.handleReadRequest(br, addr)
    case OP_ACK:
      return self.handleAck(br, addr)
    default:
      return self.handleUnknownOpcode(addr)
  }
}

func (self *Server) loop() error {
  oob := make([]byte, 1)
  for {
    maxDatagramSize := self.maxBlockSize + 2
    buf := make([]byte, maxDatagramSize+1)

    n, oobn, _, addr, err := self.socket.ReadMsgUDP(buf, oob)
    if err != nil {
      // XXX: error handling
      continue
    }

    if oobn > 0 {
      // ???
      continue
    }

    if n < 4 || n > maxDatagramSize {
      // undersize/oversize datagram, ignore
      continue
    }

    // We have a reasonable datagram.
    // TODO: IPv6 scoped addressing zones.
    self.handleDatagram(buf[0:n], addr)
  }
  return nil
}

func New(tftp_listen string, handler func(req *Request) error) (*Server, error) {
  addr, err := net.ResolveUDPAddr("udp", tftp_listen)
  if err != nil {
    return nil, err
  }

  sock, err := net.ListenUDP("udp", addr)
  if err != nil {
    return nil, err
  }

  s := &Server {
    socket: sock,
    // standard Ethernet payload MTU - IPv4 header - UDP header - TFTP header =
    // 1500 - 20 - 8 - 2 = 1470.
    // Call it 1450.
    maxBlockSize: 1450,
    ackTimeout: 1000,
    handler: handler,
    requests: make(map[string]*Request),
  }

  return s, nil
}

func (self *Server) ListenAndServe() error {
  return self.loop()
}
