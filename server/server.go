package server

import (
	"net"
	"github.com/bionicrm/implifx"
	"log"
	"encoding"
	"fmt"
	"strconv"
	"github.com/bionicrm/emulifx/ui"
	"math/rand"
	"time"
	"github.com/bionicrm/controlifx"
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

	conn, err := implifx.ListenOnPort("127.0.0.1", 0)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Println("listening at", conn.LocalAddr().String())

	_, portStr, err := net.SplitHostPort(conn.LocalAddr().String())
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
		if err := ui.ShowWindow(white, label, group, winStopCh, winActionCh); err != nil {
			log.Fatalln(err)
		}

		windowClosed = true
		conn.Close()
	}()

	target := uint64(rand.Int63())%0xffffffffffff

	for {
		n, recMsg, raddr, err := conn.Receive()
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
			var b []byte

			if alwaysRes || recMsg.Header.FrameAddress.ResRequired {
				msg.Payload = payload

				payloadSize, err := getPayloadSize()
				if err != nil {
					return err
				}
				msg.Header.Frame.Size += uint16(payloadSize)

				b, err = msg.MarshalBinary()
				if err != nil {
					return err
				}

				tx += len(b)

				if err := conn.Send(raddr, b); err != nil {
					return err
				}
			}

			if recMsg.Header.FrameAddress.AckRequired {
				msg.Header.ProtocolHeader.Type = controlifx.AcknowledgementType

				b, err = msg.MarshalBinary()
				if err != nil {
					return err
				}

				tx += len(b)

				if err := conn.Send(raddr, b); err != nil {
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

func handle(msg implifx.ReceivableLanMessage, writer writer) error {
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
	case controlifx.LightGetType:
		return lightGet(writer)
	case controlifx.LightSetColorType:
		return lightSetColor(msg, writer)
	case controlifx.LightGetPowerType:
		return lightGetPower(writer)
	case controlifx.LightSetPowerType:
		return lightSetPower(msg, writer)
	}

	return nil
}

func getService(writer writer) error {
	return writer(true, controlifx.StateServiceType, &implifx.StateServiceLanMessage{
		Service:controlifx.UdpService,
		Port:bulb.port,
	})
}

func getHostInfo(writer writer) error {
	return writer(true, controlifx.StateHostInfoType, &implifx.StateHostInfoLanMessage{})
}

func getHostFirmware(writer writer) error {
	return writer(true, controlifx.StateHostFirmwareType, &implifx.StateHostFirmwareLanMessage{
		Build:1467178139000000000,
		Version:1968197120,
	})
}

func getWifiInfo(writer writer) error {
	const Signal = 5.0118706e-6

	return writer(true, controlifx.StateWifiInfoType, &implifx.StateWifiInfoLanMessage{
		Signal:Signal,
		Tx:bulb.tx,
		Rx:bulb.rx,
	})
}

func getWifiFirmware(writer writer) error {
	return writer(true, controlifx.StateWifiFirmwareType, &implifx.StateWifiFirmwareLanMessage{
		Build:1456093684000000000,
		Version:0,
	})
}

func getPower(writer writer) error {
	var level uint16
	if bulb.powerLevel.On() {
		level = 0xffff
	}

	return writer(true, controlifx.StatePowerType, &implifx.StatePowerLanMessage{
		Level:controlifx.PowerLevel(level),
	})
}

func setPower(msg implifx.ReceivableLanMessage, writer writer) error {
	on := msg.Payload.(*implifx.SetPowerLanMessage).Level.On()

	winActionCh <- ui.PowerAction{
		On:on,
	}

	var level uint16
	if on {
		level = 0xffff
	}

	bulb.powerLevel = controlifx.PowerLevel(level)

	return writer(false, controlifx.StatePowerType, &implifx.StatePowerLanMessage{
		Level:controlifx.PowerLevel(level),
	})
}

func getLabel(writer writer) error {
	return writer(true, controlifx.StateLabelType, &implifx.StateLabelLanMessage{
		Label:controlifx.Label(bulb.label),
	})
}

func setLabel(msg implifx.ReceivableLanMessage, writer writer) error {
	bulb.label = msg.Payload.(*implifx.SetLabelLanMessage).Label

	winActionCh <- ui.LabelAction(bulb.label)

	return writer(false, controlifx.StateLabelType, &implifx.StateLabelLanMessage{
		Label:controlifx.Label(bulb.label),
	})
}

func getVersion(writer writer) error {
	var vendor, product uint32

	if bulb.white {
		vendor = controlifx.White800HighVVendorId
		product = controlifx.White800HighVProductId
	} else {
		vendor = controlifx.Color1000VendorId
		product = controlifx.Color1000ProductId
	}

	return writer(true, controlifx.StateVersionType, &implifx.StateVersionLanMessage{
		Vendor:vendor,
		Product:product,
		Version:0,
	})
}

func getInfo(writer writer) error {
	now := time.Now()

	return writer(true, controlifx.StateInfoType, &implifx.StateInfoLanMessage{
		Time:controlifx.Time(now.UnixNano()),
		Uptime:uint64(now.Sub(bulb.poweredOnAt).Nanoseconds()),
		Downtime:0,
	})
}

func getLocation(writer writer) error {
	return writer(true, controlifx.StateLocationType, &implifx.StateLocationLanMessage{
		// TODO: find documentation for location
		Location:[16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15},
		Label:controlifx.Label(bulb.group),
		UpdatedAt:controlifx.Time(bulb.groupUpdatedAt.UnixNano()),
	})
}

func getGroup(writer writer) error {
	return writer(true, controlifx.StateGroupType, &implifx.StateGroupLanMessage{
		// TODO: find documentation for group
		Group:[16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		Label:controlifx.Label(bulb.group),
		UpdatedAt:controlifx.Time(bulb.groupUpdatedAt.UnixNano()),
	})
}

func echoRequest(msg implifx.ReceivableLanMessage, writer writer) error {
	return writer(true, controlifx.EchoResponseType, &implifx.EchoResponseLanMessage{
		Payload:msg.Payload.(*implifx.EchoRequestLanMessage).Payload,
	})
}

func lightGet(writer writer) error {
	return writer(true, controlifx.LightStateType, &implifx.LightStateLanMessage{
		Color:bulb.color,
		Power:bulb.powerLevel,
		Label:bulb.label,
	})
}

func lightSetColor(msg implifx.ReceivableLanMessage, writer writer) error {
	payload := msg.Payload.(*implifx.LightSetColorLanMessage)
	bulb.color = payload.Color

	winActionCh <- ui.ColorAction{
		Color:payload.Color,
		Duration:payload.Duration,
	}

	return writer(false, controlifx.LightStateType, &implifx.LightStateLanMessage{
		Color:bulb.color,
		Power:bulb.powerLevel,
		Label:bulb.label,
	})
}

func lightGetPower(writer writer) error {
	return writer(true, controlifx.LightStatePowerType, &implifx.LightStatePowerLanMessage{
		Level:bulb.powerLevel,
	})
}

func lightSetPower(msg implifx.ReceivableLanMessage, writer writer) error {
	payload := msg.Payload.(*implifx.LightSetPowerLanMessage)

	winActionCh <- ui.PowerAction{
		On:payload.Level.On(),
		Duration:payload.Duration,
	}

	var level uint16
	if payload.Level.On() {
		level = 0xffff
	}

	bulb.powerLevel = controlifx.PowerLevel(level)

	return writer(false, controlifx.LightStatePowerType, &implifx.StatePowerLanMessage{
		Level:controlifx.PowerLevel(level),
	})
}
