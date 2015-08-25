package ipv4tps

import (
	"encoding/binary"
	"net"
)

type Netmask uint8

type IPhash uint32
type IPaddress struct {
	IP []byte
}

const IPv4_ADDRESS_LENGTH = 4

var all_ip = []byte{0, 0, 0, 0}
var IP_ALL *IPaddress = &IPaddress{IP: all_ip}
var IP_ALL_HASH = IP_ALL.Hash()

func (ip *IPaddress) Marshal() ([]byte, error) {
	return ip.IP, nil
}

func (ip *IPaddress) Hash() IPhash {
	return IPhash(binary.BigEndian.Uint32(ip.IP))
}

func (ip *IPaddress) Len() uint8 {
	return IPv4_ADDRESS_LENGTH
}

func MakeIP(ip string) *IPaddress {
	return &IPaddress{
		IP: net.ParseIP(ip)[12:],
	}
}
