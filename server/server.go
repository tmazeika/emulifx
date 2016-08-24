package server

import (
	"net"
	"github.com/bionicrm/controlifx"
	"log"
	"encoding"
	"fmt"
	"strconv"
	"github.com/bionicrm/emulifx/ui"
	"math/rand"
	"time"
	"sync"
)

var bulb lifxbulb

type lifxbulb struct {
	port  uint32
	white bool

	power bool
	color controlifx.HSBK

	txMutex sync.RWMutex
	tx      uint32
	rx      uint32

	label          string
	group          string
	groupUpdatedAt time.Time

	poweredOnAt time.Time
}

type writer func(t uint16, msg encoding.BinaryMarshaler) error

func Start(label, group string, white bool) error {
	winStopCh := make(chan interface{})
	winRgbCh := make(chan controlifx.HSBK)

	defer func() {
		winStopCh <- 0
	}()

	laddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	l, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	defer l.Close()

	fmt.Println("listening at", l.LocalAddr().String())

	_, portStr, err := net.SplitHostPort(l.LocalAddr().String())
	if err != nil {
		return err
	}

	portI, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}

	// Configure bulb.
	now := time.Now()
	bulb = lifxbulb{
		port:uint32(portI),
		white:white,
		label:label,
		group:group,
		groupUpdatedAt:now,
		poweredOnAt:now,
	}

	go func() {
		if err := ui.ShowWindow(winStopCh, winRgbCh); err != nil {
			log.Fatalln(err)
		}

		l.Close()
	}()

	target := uint64(rand.Int63())%0xffffffffffff

	for {
		b := make([]byte, controlifx.LanHeaderSize+64)
		n, raddr, err := l.ReadFromUDP(b)
		if err != nil {
			if err.(net.Error).Temporary() {
				continue
			}

			return err
		}

		bulb.rx += uint32(n)

		go func() {
			b = b[:n]

			recMsg := receivableLanMessage{}
			if err := recMsg.UnmarshalBinary(b); err != nil {
				return
			}

			handle(recMsg, func(t uint16, payload encoding.BinaryMarshaler) error {
				payloadB, err := payload.MarshalBinary()
				if err != nil {
					return err
				}

				msg := controlifx.SendableLanMessage{
					Header:controlifx.LanHeader{
						Frame:controlifx.LanHeaderFrame{
							Size:uint16(controlifx.LanHeaderSize+len(payloadB)),
							Source:recMsg.Header.Frame.Source,
						},
						FrameAddress:controlifx.LanHeaderFrameAddress{
							Target:target,
							Sequence:recMsg.Header.FrameAddress.Sequence,
						},
						ProtocolHeader:controlifx.LanHeaderProtocolHeader{
							Type:t,
						},
					},
					Payload:payload,
				}

				b, err := msg.MarshalBinary()
				if err != nil {
					return err
				}

				_, err = l.WriteTo(b, raddr)

				bulb.txMutex.Lock()
				bulb.tx += uint32(len(b))
				bulb.txMutex.Unlock()

				return err
			})
		}()
	}
}

func handle(msg receivableLanMessage, writer writer) {
	switch msg.Header.ProtocolHeader.Type {
	case controlifx.GetServiceType:
		getService(writer)
	case controlifx.GetHostInfoType:
		getHostInfo(writer)
	}
}

func getService(writer writer) {
	if err := writer(controlifx.StateServiceType, &stateServiceLanMessage{
		Service:1,
		Port:bulb.port,
	}); err != nil {
		log.Fatalln(err)
	}
}

func getHostInfo(writer writer) {
	/*bulb.txMutex.RLock()
	tx := bulb.tx
	bulb.txMutex.RUnlock()*/

	// 7.943287e-6

	if err := writer(controlifx.StateHostInfoType, &stateHostInfoLanMessage{}); err != nil {
		log.Fatalln(err)
	}
}
