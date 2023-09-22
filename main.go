package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"net"

	"github.com/kdudkov/goatak/pkg/cot"
	"github.com/kdudkov/goatak/pkg/cotproto"
)

const (
	saMulticastAddr       = "239.2.3.1:6969"
	saMulticastSensorAddr = "239.5.5.55:7171"
	chatAddr              = "224.10.10.1:17012"
	maxDatagramSize       = 8192
	magicByte             = 0xbf
)

func main() {
	ifName := flag.String("interface", "", "interface")
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", saMulticastAddr)
	if err != nil {
		log.Fatal(err)
	}
	addrChat, err := net.ResolveUDPAddr("udp", chatAddr)
	if err != nil {
		log.Fatal(err)
	}

	ch := make(chan *cotproto.TakMessage, 50)
	if *ifName != "" {
		iff, err := net.InterfaceByName(*ifName)
		if err != nil {
			panic(err)
		}
		go ListenIf(iff, addr, ch)
		go ListenIf(iff, addrChat, ch)
	} else {
		ifList, err := net.Interfaces()
		if err != nil {
			panic(err)
		}

		for i, iff := range ifList {
			if iff.Flags&net.FlagUp != 0 && iff.Flags&net.FlagMulticast != 0 {
				go ListenIf(&ifList[i], addr, ch)
				go ListenIf(&ifList[i], addrChat, ch)
			}
		}
	}

	for msg := range ch {
		b, err := json.Marshal(msg)
		if err == nil {
			fmt.Println(string(b))
		}
	}
}

func ListenIf(ifi *net.Interface, addr *net.UDPAddr, ch chan *cotproto.TakMessage) {
	conn, err := net.ListenMulticastUDP("udp", ifi, addr)
	if err != nil {
		return
	}

	_ = conn.SetReadBuffer(maxDatagramSize)
	buf := make([]byte, maxDatagramSize)

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
		}

		if n < 4 {
			continue
		}

		if buf[0] == magicByte && buf[2] == magicByte {
			if buf[1] == 1 {
				msg := new(cotproto.TakMessage)
				err = proto.Unmarshal(buf[3:n], msg)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				ch <- msg
			} else {
				ev := &cot.Event{}
				err = xml.Unmarshal(buf[3:n], ev)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}

				msg, _ := cot.EventToProto(ev)
				ch <- msg
			}
		}
	}
}
