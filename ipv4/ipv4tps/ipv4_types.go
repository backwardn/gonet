package ipv4tps

import (
	"net"
)

type Netmask uint8

type IPaddress string

func (ip *IPaddress) Marshal() ([]byte, error) {
	x := net.ParseIP(string(*ip))
	return x[12:], nil
}

func MakeIP(ip string) *IPaddress {
	p := IPaddress(ip)
	return &p
}
