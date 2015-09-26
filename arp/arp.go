package arp

import (
	"errors"
	"network/ethernet"

	"reflect"
	"time"

	"github.com/hsheth2/logs"
	"github.com/hsheth2/notifiers"
)

type ARP_Manager struct {
	read          ethernet.Ethernet_Reader
	ethtp_manager map[ethernet.EtherType](ARP_Protocol_Dealer)
}

var GlobalARP_Manager *ARP_Manager = func() *ARP_Manager {
	am, err := NewARP_Manager(ethernet.GlobalNetworkReadManager)
	if err != nil {
		logs.Error.Fatalln(err)
	}
	return am
}()

func NewARP_Manager(in *ethernet.Network_Read_Manager) (*ARP_Manager, error) {
	read, err := in.Bind(ethernet.ETHERTYPE_ARP)
	if err != nil {
		return nil, err
	}

	am := &ARP_Manager{
		read:          read,
		ethtp_manager: make(map[ethernet.EtherType](ARP_Protocol_Dealer)),
	}

	go am.dealer()

	return am, nil
}

func (am *ARP_Manager) Register(tp ethernet.EtherType, arppd ARP_Protocol_Dealer) error {
	if tp == ethernet.ETHERTYPE_ARP {
		return errors.New("ARP Manager: cannot bind to ARP ethertype")
	}
	if _, ok := am.ethtp_manager[tp]; ok {
		return errors.New("ARP Manager: ethertype already bound to")
	}
	am.ethtp_manager[tp] = arppd
	return nil
}

// TODO make unregister function

func (am *ARP_Manager) dealer() {
	for {
		header, err := am.read.Read()
		if err != nil {
			logs.Error.Println(err)
			continue
		}
		data := header.Packet
		packet := ParseARP_Packet_General(data)

		if pd, ok := am.ethtp_manager[packet.ptype]; ok && packet.htype == ARP_HTYPE_ETHERNET {
			packet = ParseARP_Packet_Type(data, packet, pd)
			//logs.Trace.Println("ARP packet:", packet)
			pd.Add(packet.spa, packet.sha)
			if packet.oper == ARP_OPER_REQUEST {
				////ch logs.Trace.Println("Got ARP Request")
				if reflect.DeepEqual(packet.tpa, pd.GetAddress()) {
					reply := &ARP_Packet{
						htype: packet.htype,
						ptype: packet.ptype,
						hlen:  packet.hlen,
						plen:  packet.plen,
						oper:  ARP_OPER_REPLY,
						sha:   ethernet.External_mac_address,
						spa:   pd.GetAddress(),
						tha:   packet.sha,
						tpa:   packet.spa,
					}
					rp, err := reply.MarshalPacket()
					if err != nil {
						logs.Warn.Println("MarshalPacket failed; dropping ARP request")
						continue
					}
					_, err = ethernet.EthernetWriteOne(reply.tha, ethernet.ETHERTYPE_ARP, rp)
					if err != nil {
						logs.Warn.Println("Failed to send ARP response; dropping request packet")
						continue
					}
					//logs.Trace.Println("Replied to ARP request")
				} else {
					logs.Warn.Println("Ignoring ARP request with a different target protocol address")
					continue
				}
			} else if packet.oper == ARP_OPER_REPLY {
				//logs.Trace.Println("Got ARP Reply")
				// signal is sent in the Add function
			} else {
				logs.Warn.Println("Dropping ARP packet for bad operation")
			}
		}
	}
}

func (am *ARP_Manager) Request(tp ethernet.EtherType, raddr ARP_Protocol_Address) (*ethernet.MAC_Address, error) {
	if pd, ok := am.ethtp_manager[tp]; ok {
		// prepare request
		requestPacket := &ARP_Packet{
			htype: ARP_HTYPE_ETHERNET,
			ptype: tp,
			hlen:  ARP_HLEN_ETHERNET,
			plen:  raddr.Len(),
			oper:  ARP_OPER_REQUEST,
			sha:   ethernet.External_mac_address,
			spa:   pd.GetAddress(),
			tha:   ethernet.External_bcast_address,
			tpa:   raddr,
		}

		// make request
		request, err := requestPacket.MarshalPacket()
		if err != nil {
			return nil, err
		}

		// send request
		_, err = ethernet.EthernetWriteOne(requestPacket.tha, ethernet.ETHERTYPE_ARP, request)
		if err != nil {
			return nil, err
		}

		// register for reply
		reply := pd.GetReplyNotifier().Register(2)
		defer pd.GetReplyNotifier().Unregister(reply)

		// wait for reply
		timeout := time.NewTimer(ARP_REQUEST_TIMEOUT)
		for {
			select {
			case <-timeout.C:
				return nil, errors.New("ARP request timed out")
			case <-reply:
				// check if entry is there now; otherwise, wait for another reply
				ans, err := pd.Lookup(raddr)
				if err == nil {
					return ans, nil
				}
			}
		}
	} else {
		return nil, errors.New("No ARP_Protocol_Dealer registered for given EtherType")
	}
}

type ARP_Protocol_Dealer interface {
	Lookup(ARP_Protocol_Address) (*ethernet.MAC_Address, error)
	Request(ARP_Protocol_Address) (*ethernet.MAC_Address, error)
	// TODO add discover (probe) function to broadcast ARP requests
	// TODO support ARP announcements
	Add(ARP_Protocol_Address, *ethernet.MAC_Address) error
	GetReplyNotifier() *notifiers.Notifier
	Unmarshal([]byte) ARP_Protocol_Address
	GetAddress() ARP_Protocol_Address
}

type ARP_Protocol_Address interface {
	Marshal() ([]byte, error)
	Len() uint8
}
