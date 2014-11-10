package tftpsrv
import "net"
import "time"
import "bytes"

type Request struct {
  // Peer address
  addr *net.UDPAddr
  ackChannel chan uint16
  Filename string
  Mode string
  server *Server
  blockSize int
  blockBuf bytes.Buffer
  blockNum uint16
  terminated bool
  options map[string]string
}

func (req *Request) flushBlock(isFinal bool) error {
  if req.blockBuf.Len() == 0 && !isFinal {
    return nil
  }

  loop:
  for {
    if !isFinal && req.blockBuf.Len() != req.blockSize {
      panic("unexpected short block")
    }
    err := req.server.sendTftpDataPacket(req.addr, req.blockNum, req.blockBuf.Bytes())
    if err != nil {
      return err
    }

    ta := time.After(time.Duration(req.server.ackTimeout)*time.Millisecond)
    select {
      case brctl := <-req.ackChannel:
        if brctl == req.blockNum {
          break loop
        }
      case <-ta:
    }
  }

  req.blockBuf.Truncate(0)
  req.blockNum += 1
  return nil
}

func (req *Request) setOption(k, v string) {
  req.options[k] = v
}

func (req *Request) ClientAddress() net.IP {
  return req.addr.IP
}

func (req *Request) Write(p []byte) (int, error) {
  L := len(p)
  if L > req.blockSize {
    L = req.blockSize
  }
  L -= req.blockBuf.Len()
  if L > 0 {
    req.blockBuf.Write(p[0:L])
  }

  if req.blockBuf.Len() < req.blockSize {
    return L, nil
  }

  err := req.flushBlock(false)
  if err != nil {
    return 0, err
  }

  return L, nil
}

func (req *Request) terminate() {
  delete(req.server.requests, req.name())
  req.terminated = true
}

func (req *Request) Close() {
  if req.terminated {
    return
  }
  req.flushBlock(true)
  req.terminate()
}

func (req *Request) WriteError(num uint16, msg string) {
  req.server.sendTftpErrorPacket(req.addr, num, msg)
  req.terminate()
}

func (self *Request) name() string {
  // TODO: Currently we identify requests by a string formed from the address.
  // ...
  return nameFromAddr(self.addr)
}

// This is run in a per-request goroutine. Blocking is OK.
func (self *Request) loop(s *Server) {
  // TODO: TFTP Option Negotiation support.
  // At this point we could perform option negotiation by sending OACK etc.

  // Ready for write
  s.handler(self)
  // TODO: Error handling.
}
