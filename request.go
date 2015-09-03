package tftpsrv

import "net"
import "time"
import "bytes"
import "fmt"
import "strconv"
import "github.com/hlandau/degoutils/log"

// Represents a TFTP request.
type Request struct {
	Filename string // The requested filename.
	Mode     string // The request mode. Can usually be ignored.

	addr       *net.UDPAddr // Peer address
	ackChannel chan uint16  // Channel which receives acknowledgement numbers.
	server     *Server
	blockSize  uint16
	blockBuf   bytes.Buffer
	blockNum   uint16
	terminated bool
	options    map[string]string
}

func (req *Request) sendAndWaitForAck(txFunc func() error, blockNum uint16) error {
	t := time.Now()

loop:
	for {
		err := txFunc()
		if err != nil {
			return err
		}

		select {
		case brctl := <-req.ackChannel:
			if brctl == blockNum {
				break loop
			}
		case <-time.After(req.server.RetransmissionTimeout):
			if time.Now().After(t.Add(req.server.RequestTimeout)) {
				req.terminate()
				return ErrTimedOut
			}
		}
	}

	return nil
}

func (req *Request) fbSendData() error {
	return req.server.sendTftpDataPacket(req.addr, req.blockNum, req.blockBuf.Bytes())
}

func (req *Request) flushBlock(isFinal bool) error {
	if req.blockBuf.Len() == 0 && !isFinal {
		return nil
	}

	if !isFinal && req.blockBuf.Len() != int(req.blockSize) {
		panic("unexpected short block")
	}

	err := req.sendAndWaitForAck(req.fbSendData, req.blockNum)

	if err != nil {
		return err
	}

	req.blockBuf.Truncate(0)
	req.blockNum++
	return nil
}

func (req *Request) setOption(k, v string) error {
	switch k {
	case "blksize":
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return err
		}

		return req.setBlockSize(uint16(n))

	default:
		// not supported
		return fmt.Errorf("not supported")
	}
}

func (req *Request) setBlockSize(blockSize uint16) error {
	if blockSize < 8 || blockSize > 65464 {
		return fmt.Errorf("invalid block size: %d not in 8 <= blkSize <= 65464", blockSize)
	}
	req.blockSize = blockSize
	req.options["blksize"] = strconv.FormatUint(uint64(blockSize), 10)
	return nil
}

// Returns the address of the client which initiated the TFTP transfer.
func (req *Request) ClientAddress() net.UDPAddr {
	return *req.addr
}

// Request already closed.
var ErrClosed = fmt.Errorf("Request already closed")

// Request timed out.
var ErrTimedOut = fmt.Errorf("Request timed out")

// Writes bytes which represent data from the file requested. The data provided
// is logically appended to the data provided in previous calls to Write. There
// are no particular requirements on the size of the buffer passed, and the
// size passed may vary between calls to Write.
//
// The amount of data written is returned. Note that not all data provided may
// be written. Returns ErrClosed if the request has already been terminated.
func (req *Request) write(data []byte) (int, error) {
	if req.terminated {
		return 0, ErrClosed
	}

	L := len(data)

	Lmax := int(req.blockSize) - req.blockBuf.Len()
	if L > Lmax {
		L = Lmax
	}

	if L > 0 {
		req.blockBuf.Write(data[0:L])
	}

	if req.blockBuf.Len() < int(req.blockSize) {
		return L, nil
	}

	err := req.flushBlock(false)
	if err != nil {
		return 0, err
	}

	return L, nil
}

// Writes bytes which represent data from the file requested. The data provided
// is logically appended to the data provided in previous calls to Write. There
// are no particular requirements on the size of the buffer passed, and the
// size passed may vary between calls to Write. In other words, this method
// implements the semantics of io.Writer.
//
// Returns ErrClosed if the request has already been terminated.
func (req *Request) Write(data []byte) (int, error) {
	log.Info("WR: data len=", len(data))

	n := 0
	for len(data) > 0 {
		nn, err := req.write(data)
		log.Info("    r ", nn, "  / ", len(data))
		n += nn
		if err != nil {
			return n, err
		}
		data = data[nn:]
	}
	return n, nil
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

	err := req.finishOptionNegotiation()
	if err != nil {
		return
	}

	// Ready for write
	req.server.ReadHandler(req)
}

func (req *Request) finishOptionNegotiation() error {
	if len(req.options) == 0 {
		return nil
	}

	err := req.sendAndWaitForAck(func() error {
		return req.server.sendTftpOptNegPacket(req.addr, req.options)
	}, 0)
	if err != nil {
		return err
	}

	return nil
}

// © 2014 Hugo Landau <hlandau@devever.net>    GPLv3 or later
