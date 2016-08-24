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
)

type (
	lifxbulb struct {
		port  uint32
		white bool

		powerLevel controlifx.PowerLevel
		color      controlifx.HSBK

		tx uint32
		rx uint32

		label          controlifx.Label
		group          string
		groupUpdatedAt time.Time

		poweredOnAt time.Time
	}

	writer func(alwaysRes bool, t uint16, msg encoding.BinaryMarshaler) error
)

var (
	bulb lifxbulb

	winStopCh   = make(chan interface{})
	winActionCh = make(chan interface{})
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
		label:controlifx.Label(label),
		group:group,
		groupUpdatedAt:now,
		poweredOnAt:now,
	}

	var windowClosed bool

	go func() {
		if err := ui.ShowWindow(label, group, winStopCh, winActionCh); err != nil {
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

		bulb.rx += uint32(n)
		b = b[:n]

		recMsg := receivableLanMessage{}
		if err := recMsg.UnmarshalBinary(b); err != nil {
			continue
		}

		if err := handle(recMsg, func(alwaysRes bool, t uint16, payload encoding.BinaryMarshaler) error {
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

			getPayloadSize := func() (int, error) {
				b, err := payload.MarshalBinary()
				if err != nil {
					return 0, err
				}

				return len(b), nil
			}
			var tx int

			if alwaysRes || recMsg.Header.FrameAddress.ResRequired {
				msg.Payload = payload

				payloadSize, err := getPayloadSize()
				if err != nil {
					return err
				}
				msg.Header.Frame.Size += uint16(payloadSize)

				b, err := msg.MarshalBinary()
				if err != nil {
					return err
				}

				tx += len(b)

				if _, err := l.WriteTo(b, raddr); err != nil {
					return err
				}
			}

			if recMsg.Header.FrameAddress.AckRequired {
				msg.Header.ProtocolHeader.Type = controlifx.AcknowledgementType

				b, err := msg.MarshalBinary()
				if err != nil {
					return err
				}

				tx += len(b)

				if _, err := l.WriteTo(b, raddr); err != nil {
					return err
				}
			}

			bulb.tx += uint32(len(b))

			return err
		}); err != nil {
			log.Println(err)
		}
	}
}

func handle(msg receivableLanMessage, writer writer) error {
	switch msg.Header.ProtocolHeader.Type {
	case controlifx.GetServiceType:
		return getService(writer)
	case controlifx.GetHostInfoType:
		return getHostInfo(writer)
	case controlifx.GetHostFirmwareType:
		return getHostFirmware(writer)
	case controlifx.GetWifiInfoType:
		return getWifiInfo(writer)
	case controlifx.GetWifiFirmwareType:
		return getWifiFirmware(writer)
	case controlifx.GetPowerType:
		return getPower(writer)
	case controlifx.SetPowerType:
		return setPower(msg, writer)
	case controlifx.GetLabelType:
		return getLabel(writer)
	case controlifx.SetLabelType:
		return setLabel(msg, writer)
	case controlifx.GetVersionType:
		return getVersion(writer)
	case controlifx.GetInfoType:
		return getInfo(writer)
	case controlifx.GetLocationType:
		return getLocation(writer)
	case controlifx.GetGroupType:
		return getGroup(writer)
	case controlifx.EchoRequestType:
		return echoRequest(msg, writer)
	}

	return nil
}

func getService(writer writer) error {
	return writer(true, controlifx.StateServiceType, &stateServiceLanMessage{
		Service:1,
		Port:bulb.port,
	})
}

func getHostInfo(writer writer) error {
	return writer(true, controlifx.StateHostInfoType, &stateHostInfoLanMessage{})
}

func getHostFirmware(writer writer) error {
	return writer(true, controlifx.StateHostFirmwareType, &stateHostFirmwareLanMessage{
		Build:1300233600000000000,
		Version:1,
	})
}

func getWifiInfo(writer writer) error {
	const Signal = 7.943287e-6

	return writer(true, controlifx.StateWifiInfoType, &stateWifiInfoLanMessage{
		Signal:Signal,
		Tx:bulb.tx,
		Rx:bulb.rx,
	})
}

func getWifiFirmware(writer writer) error {
	return writer(true, controlifx.StateWifiFirmwareType, &stateWifiFirmwareLanMessage{
		Build:1300233600000000000,
		Version:1,
	})
}

func getPower(writer writer) error {
	var level uint16
	if bulb.powerLevel.On() {
		level = 0xffff
	}

	return writer(true, controlifx.StatePowerType, &statePowerLanMessage{
		Level:controlifx.PowerLevel(level),
	})
}

func setPower(msg receivableLanMessage, writer writer) error {
	on := msg.Payload.(*setPowerLanMessage).Level.On()

	winActionCh <- ui.PowerAction{
		On:on,
	}

	var level uint16
	if on {
		level = 0xffff
	}

	bulb.powerLevel = controlifx.PowerLevel(level)

	return writer(false, controlifx.StatePowerType, &statePowerLanMessage{
		Level:controlifx.PowerLevel(level),
	})
}

func getLabel(writer writer) error {
	return writer(true, controlifx.StateLabelType, &stateLabelLanMessage{
		Label:controlifx.Label(bulb.label),
	})
}

func setLabel(msg receivableLanMessage, writer writer) error {
	bulb.label = msg.Payload.(*setLabelLanMessage).Label

	winActionCh <- ui.LabelAction(bulb.label)

	return writer(false, controlifx.StateLabelType, &stateLabelLanMessage{
		Label:controlifx.Label(bulb.label),
	})
}

func getVersion(writer writer) error {
	return writer(true, controlifx.StateVersionType, &stateVersionLanMessage{
		Vendor:0,
		Product:1,
		Version:2,
	})
}

func getInfo(writer writer) error {
	now := time.Now()

	return writer(true, controlifx.StateInfoType, &stateInfoLanMessage{
		Time:controlifx.Time(now.UnixNano()),
		Uptime:uint64(now.Sub(bulb.poweredOnAt).Nanoseconds()),
		Downtime:0,
	})
}

func getLocation(writer writer) error {
	return writer(true, controlifx.StateLocationType, &stateLocationLanMessage{
		Location:[16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15},
		Label:controlifx.Label(bulb.group),
		UpdatedAt:controlifx.Time(bulb.groupUpdatedAt.UnixNano()),
	})
}

func getGroup(writer writer) error {
	return writer(true, controlifx.StateGroupType, &stateGroupLanMessage{
		Group:[16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		Label:controlifx.Label(bulb.group),
		UpdatedAt:controlifx.Time(bulb.groupUpdatedAt.UnixNano()),
	})
}

func echoRequest(msg receivableLanMessage, writer writer) error {
	return writer(true, controlifx.EchoResponseType, &echoResponseLanMessage{
		Payload:msg.Payload.(*echoRequestLanMessage).Payload,
	})
}
