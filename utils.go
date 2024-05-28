package tftpsrv

import (
	"fmt"
	"net"
)

func nameFromAddr(addr *net.UDPAddr) string {
	return fmt.Sprintf("%s/%d/%s", addr.String(), addr.Port, addr.Zone)
}

func cstrToString(b []byte) string {
	return string(b[0 : len(b)-1])
}
