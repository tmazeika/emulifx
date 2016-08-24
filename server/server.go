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

type (
	lifxbulb struct {
		port  uint32
		white bool

		power bool
		color controlifx.HSBK

		txMutex sync.RWMutex
		tx      uint32
		rxMutex sync.RWMutex
		rx      uint32

		label          string
		group          string
		groupUpdatedAt time.Time

		poweredOnAt time.Time
	}

	writer func(alwaysRes bool, t uint16, msg encoding.BinaryMarshaler) error
)

var (
	bulb lifxbulb

	winStopCh  = make(chan interface{})
	winPowerCh = make(chan ui.PowerAction)
	winColorCh = make(chan ui.ColorAction)
)

func Start(label, group string, white bool) error {
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

	var windowClosed bool

	go func() {
		if err := ui.ShowWindow(winStopCh, winPowerCh, winColorCh); err != nil {
			log.Fatalln(err)
		}

		windowClosed = true
		l.Close()
	}()

	target := uint64(rand.Int63())%0xffffffffffff

	for {
		b := make([]byte, controlifx.LanHeaderSize+64)
		n, raddr, err := l.ReadFromUDP(b)
		if err != nil {
			if windowClosed {
				return nil
			}
			if err.(net.Error).Temporary() {
				continue
			}

			return err
		}

		bulb.rxMutex.Lock()
		bulb.rx += uint32(n)
		bulb.rxMutex.Unlock()

		go func() {
			b = b[:n]

			recMsg := receivableLanMessage{}
			if err := recMsg.UnmarshalBinary(b); err != nil {
				return
			}

			handle(recMsg, func(alwaysRes bool, t uint16, payload encoding.BinaryMarshaler) error {
				msg := controlifx.SendableLanMessage{
					Header:controlifx.LanHeader{
						Frame:controlifx.LanHeaderFrame{
							Size:controlifx.LanHeaderSize,
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
				}

				msgToBytes := func() ([]byte, error) {
					b, err := msg.MarshalBinary()
					if err != nil {
						return []byte{}, err
					}

					return b, nil
				}
				getPayloadSize := func() (int, error) {
					b, err := payload.MarshalBinary()
					if err != nil {
						return 0, err
					}

					return len(b), nil
				}
				var tx int

				if recMsg.Header.FrameAddress.AckRequired {
					b, err := msgToBytes()
					if err != nil {
						return err
					}

					tx += len(b)

					if _, err := l.WriteTo(b, raddr); err != nil {
						return err
					}
				}

				if alwaysRes || recMsg.Header.FrameAddress.ResRequired {
					msg.Payload = payload

					payloadSize, err := getPayloadSize()
					if err != nil {
						return err
					}
					msg.Header.Frame.Size += uint16(payloadSize)

					b, err := msgToBytes()
					if err != nil {
						return err
					}

					tx += len(b)

					if _, err := l.WriteTo(b, raddr); err != nil {
						return err
					}
				}

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
	case controlifx.GetHostFirmwareType:
		getHostFirmware(writer)
	case controlifx.GetWifiInfoType:
		getWifiInfo(writer)
	case controlifx.GetWifiFirmwareType:
		getWifiFirmware(writer)
	case controlifx.GetPowerType:
		getPower(writer)
	case controlifx.SetPowerType:
		setPower(msg, writer)
	}
}

func getService(writer writer) {
	if err := writer(true, controlifx.StateServiceType, &stateServiceLanMessage{
		Service:1,
		Port:bulb.port,
	}); err != nil {
		log.Fatalln(err)
	}
}

func getHostInfo(writer writer) {
	if err := writer(true, controlifx.StateHostInfoType, &stateHostInfoLanMessage{}); err != nil {
		log.Fatalln(err)
	}
}

func getHostFirmware(writer writer) {
	if err := writer(true, controlifx.StateHostFirmwareType, &stateHostFirmwareLanMessage{
		Build:1300233600000000000,
		Version:1,
	}); err != nil {
		log.Fatalln(err)
	}
}

func getWifiInfo(writer writer) {
	const signal = 7.943287e-6

	// Tx.
	bulb.txMutex.RLock()
	tx := bulb.tx
	bulb.txMutex.RUnlock()

	// Rx.
	bulb.rxMutex.RLock()
	rx := bulb.rx
	bulb.rxMutex.RUnlock()

	if err := writer(true, controlifx.StateWifiInfoType, &stateWifiInfoLanMessage{
		Signal:signal,
		Tx:tx,
		Rx:rx,
	}); err != nil {
		log.Fatalln(err)
	}
}

func getWifiFirmware(writer writer) {
	if err := writer(true, controlifx.StateWifiFirmwareType, &stateWifiFirmwareLanMessage{
		Build:1300233600000000000,
		Version:1,
	}); err != nil {
		log.Fatalln(err)
	}
}

func getPower(writer writer) {
	var level uint16
	if bulb.power {
		level = 0xffff
	}

	if err := writer(true, controlifx.StatePowerType, &statePowerLanMessage{
		Level:controlifx.PowerLevel(level),
	}); err != nil {
		log.Fatalln(err)
	}
}

func setPower(msg receivableLanMessage, writer writer) {
	on := (uint16(msg.Payload.(*setPowerLanMessage).Level) == 0xffff)

	winPowerCh <- ui.PowerAction{
		On:on,
	}

	var level uint16
	if bulb.power {
		level = 0xffff
	}

	if err := writer(false, controlifx.StatePowerType, &statePowerLanMessage{
		Level:controlifx.PowerLevel(level),
	}); err != nil {
		log.Fatalln(err)
	}
}
