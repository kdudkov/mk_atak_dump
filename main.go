package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/protobuf/proto"

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

var (
	gitRevision = "unknown"
	gitBranch   = "unknown"
)

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
				ch <- msg.TakMessage
			}
		}
	}
}

func getVersion() string {
	if gitBranch != "master" && gitBranch != "unknown" {
		return fmt.Sprintf("%s:%s", gitBranch, gitRevision)
	}

	return gitRevision
}

func main() {
	version := flag.Bool("version", false, "show version")
	ifName := flag.String("interface", "", "interface")
	ver := flag.String("ver", "2", "version (1 or 2)")
	flag.Parse()

	if *version {
		fmt.Println(getVersion())
		return
	}

	if *ver != "1" && *ver != "2" {
		fmt.Println("version must be 1 or 2")
		return
	}

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
		switch *ver {
		case "1":
			msg1 := cot.ProtoToEvent(msg)
			b, err := xml.Marshal(msg1)
			if err == nil {
				fmt.Println(string(b))
			} else {
				fmt.Println(err)
			}

		case "2":
			b, err := json.Marshal(msg)
			if err == nil {
				fmt.Println(string(b))
			} else {
				fmt.Println(err)
			}
		}
	}
}
