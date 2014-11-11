package tftpsrv
import "net"
import "time"
import "bytes"
import "fmt"

// Represents a TFTP request.
type Request struct {
  Filename string         // The requested filename.
  Mode string             // The request mode. Can usually be ignored.

  addr *net.UDPAddr       // Peer address
  ackChannel chan uint16  // Channel which receives acknowledgement numbers.
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

  t := time.Now()

loop:
  for {
    if !isFinal && req.blockBuf.Len() != req.blockSize {
      panic("unexpected short block")
    }
    err := req.server.sendTftpDataPacket(req.addr, req.blockNum, req.blockBuf.Bytes())
    if err != nil {
      return err
    }

    select {
      case brctl := <-req.ackChannel:
        if brctl == req.blockNum {
          break loop
        }
      case <-time.After(req.server.RetransmissionTimeout):
        if time.Now().After(t.Add(req.server.RequestTimeout)) {
          req.terminate()
          return ErrTimedOut
        }
    }
  }

  req.blockBuf.Truncate(0)
  req.blockNum += 1
  return nil
}

func (req *Request) setOption(k, v string) {
  req.options[k] = v
}

// Returns the address of the client which initiated the TFTP transfer.
func (req *Request) ClientAddress() net.UDPAddr {
  return *req.addr
}

var ErrClosed   = fmt.Errorf("Request already closed")
var ErrTimedOut = fmt.Errorf("Request timed out")

// Writes bytes which represent data from the file requested. The data provided
// is logically appended to the data provided in previous calls to Write. There
// are no particular requirements on the size of the buffer passed, and the
// size passed may vary between calls to Write.
//
// The amount of data written is returned. Note that not all data provided may
// be written. Returns ErrClosed if the request has already been terminated.
func (req *Request) Write(data []byte) (int, error) {
	if req.terminated {
		return 0, ErrClosed
	}

  L := len(data)
  if L > req.blockSize {
    L = req.blockSize
  }
  L -= req.blockBuf.Len()
  if L > 0 {
    req.blockBuf.Write(data[0:L])
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

// This method must be called when the TFTP transfer has completed
// successfully. The request enters a terminated state in which further
// operations are not possible. Multiple calls to this method are
// inconsequential.
func (req *Request) Close() {
  if req.terminated {
    return
  }
  req.flushBlock(true)
  req.terminate()
}

// Terminates the TFTP transfer with an error code. The request enters a
// terminated state in which further operations are not possible. Further calls
// to this method are inconsequential.
func (req *Request) WriteError(errNum Error, msg string) {
  if req.terminated {
    return
  }
  req.server.sendTftpErrorPacket(req.addr, errNum, msg)
  req.terminate()
}

func (req *Request) name() string {
  // TODO: Currently we identify requests by a string formed from the address.
  // ...
  return nameFromAddr(req.addr)
}

// This is run in a per-request goroutine. Blocking is OK.
func (req *Request) loop() {
  // TODO: TFTP Option Negotiation support.
  // At this point we could perform option negotiation by sending OACK etc.

  // Ready for write
  req.server.ReadHandler(req)
}
