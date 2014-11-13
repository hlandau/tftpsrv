package tftpsrv

import "fmt"
import "net"

func nameFromAddr(addr *net.UDPAddr) string {
	return fmt.Sprintf("%s/%u/%s", addr.String(), addr.Port, addr.Zone)
}

func cstrToString(b []byte) string {
	return string(b[0 : len(b)-1])
}

// © 2014 Hugo Landau <hlandau@devever.net>    GPLv3 or later
