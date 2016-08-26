package server

import (
	"encoding"
	"fmt"
	"github.com/bionicrm/emulifx/ui"
	"gopkg.in/golifx/controlifx.v1"
	"gopkg.in/golifx/implifx.v1"
	"log"
	"math/rand"
	"net"
	"time"
)

type (
	lifxbulb struct {
		port  uint16
		white bool

		poweredOn   bool
		poweredOnAt time.Time
		color       controlifx.HSBK

		tx uint32
		rx uint32

		label          string
		group          string
		groupUpdatedAt time.Time
	}

	writer func(always bool, t uint16, msg encoding.BinaryMarshaler) error
)

var (
	bulb lifxbulb

	winStopCh   = make(chan interface{})
	winActionCh = make(chan interface{})
)

func Start(addr, label, group string, white bool) error {
	defer func() {
		winStopCh <- 0
	}()

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	conn, err := implifx.ListenOnOtherPort(host, portStr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Mock MAC.
	conn.Mac = uint64(rand.Int63()) % 0xffffffffffff

	fmt.Println("Listening at", conn.LocalAddr().String())

	// Configure bulb.
	now := time.Now()
	bulb = lifxbulb{
		port:        conn.Port(),
		white:       white,
		poweredOnAt: now,
		color: controlifx.HSBK{
			Kelvin: 3500,
		},
		label:          label,
		group:          group,
		groupUpdatedAt: now,
	}

	var windowClosed bool

	go func() {
		if err := ui.ShowWindow(white, label, group, conn.LocalAddr().String(), winStopCh, winActionCh); err != nil {
			log.Fatalln(err)
		}

		windowClosed = true
		conn.Close()
	}()

	for {
		n, raddr, recMsg, err := conn.Receive()
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

		if err := handle(recMsg, func(always bool, t uint16, payload encoding.BinaryMarshaler) error {
			tx, err := conn.Respond(always, raddr, recMsg, t, payload)
			bulb.tx += uint32(tx)

			return err
		}); err != nil {
			log.Println(err)
		}
	}
}

func (o lifxbulb) PowerLevel() uint16 {
	if o.poweredOn {
		return 0xffff
	}

	return 0
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
		Service: controlifx.UdpService,
		Port:    uint32(bulb.port),
	})
}

func getHostInfo(writer writer) error {
	return writer(true, controlifx.StateHostInfoType, &implifx.StateHostInfoLanMessage{})
}

func getHostFirmware(writer writer) error {
	return writer(true, controlifx.StateHostFirmwareType, &implifx.StateHostFirmwareLanMessage{
		Build:   1467178139000000000,
		Version: 1968197120,
	})
}

func getWifiInfo(writer writer) error {
	return writer(true, controlifx.StateWifiInfoType, &implifx.StateWifiInfoLanMessage{
		Signal: 5.0118706e-6,
		Tx:     bulb.tx,
		Rx:     bulb.rx,
	})
}

func getWifiFirmware(writer writer) error {
	return writer(true, controlifx.StateWifiFirmwareType, &implifx.StateWifiFirmwareLanMessage{
		Build:   1456093684000000000,
		Version: 0,
	})
}

func getPower(writer writer) error {
	return writer(true, controlifx.StatePowerType, &implifx.StatePowerLanMessage{
		Level: bulb.PowerLevel(),
	})
}

func setPower(msg implifx.ReceivableLanMessage, writer writer) error {
	responsePayload := &implifx.StatePowerLanMessage{
		Level: bulb.PowerLevel(),
	}
	bulb.poweredOn = msg.Payload.(*implifx.SetPowerLanMessage).Level == 0xffff

	winActionCh <- ui.PowerAction{
		On: bulb.poweredOn,
	}

	return writer(false, controlifx.StatePowerType, responsePayload)
}

func getLabel(writer writer) error {
	return writer(true, controlifx.StateLabelType, &implifx.StateLabelLanMessage{
		Label: bulb.label,
	})
}

func setLabel(msg implifx.ReceivableLanMessage, writer writer) error {
	bulb.label = msg.Payload.(*implifx.SetLabelLanMessage).Label

	winActionCh <- ui.LabelAction(bulb.label)

	return writer(false, controlifx.StateLabelType, &implifx.StateLabelLanMessage{
		Label: bulb.label,
	})
}

func getVersion(writer writer) error {
	responsePayload := &implifx.StateVersionLanMessage{
		Version: 0,
	}

	if bulb.white {
		responsePayload.Vendor = controlifx.White800HighVVendorId
		responsePayload.Product = controlifx.White800HighVProductId
	} else {
		responsePayload.Vendor = controlifx.Color1000VendorId
		responsePayload.Product = controlifx.Color1000ProductId
	}

	return writer(true, controlifx.StateVersionType, responsePayload)
}

func getInfo(writer writer) error {
	now := time.Now()

	return writer(true, controlifx.StateInfoType, &implifx.StateInfoLanMessage{
		Time:     uint64(now.UnixNano()),
		Uptime:   uint64(now.Sub(bulb.poweredOnAt).Nanoseconds()),
		Downtime: 0,
	})
}

func getLocation(writer writer) error {
	return writer(true, controlifx.StateLocationType, &implifx.StateLocationLanMessage{
		// TODO: find documentation for location
		Location:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15},
		Label:     bulb.group,
		UpdatedAt: uint64(bulb.groupUpdatedAt.UnixNano()),
	})
}

func getGroup(writer writer) error {
	return writer(true, controlifx.StateGroupType, &implifx.StateGroupLanMessage{
		// TODO: find documentation for group
		Group:     [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		Label:     bulb.group,
		UpdatedAt: uint64(bulb.groupUpdatedAt.UnixNano()),
	})
}

func echoRequest(msg implifx.ReceivableLanMessage, writer writer) error {
	return writer(true, controlifx.EchoResponseType, &implifx.EchoResponseLanMessage{
		Payload: msg.Payload.(*implifx.EchoRequestLanMessage).Payload,
	})
}

func lightGet(writer writer) error {
	return writer(true, controlifx.LightStateType, &implifx.LightStateLanMessage{
		Color: bulb.color,
		Power: bulb.PowerLevel(),
		Label: bulb.label,
	})
}

func lightSetColor(msg implifx.ReceivableLanMessage, writer writer) error {
	responsePayload := &implifx.LightStateLanMessage{
		Color: bulb.color,
		Power: bulb.PowerLevel(),
		Label: bulb.label,
	}
	payload := msg.Payload.(*implifx.LightSetColorLanMessage)
	bulb.color = payload.Color

	winActionCh <- ui.ColorAction{
		Color:    payload.Color,
		Duration: payload.Duration,
	}

	return writer(false, controlifx.LightStateType, responsePayload)
}

func lightGetPower(writer writer) error {
	return writer(true, controlifx.LightStatePowerType, &implifx.LightStatePowerLanMessage{
		Level: bulb.PowerLevel(),
	})
}

func lightSetPower(msg implifx.ReceivableLanMessage, writer writer) error {
	responsePayload := &implifx.StatePowerLanMessage{
		Level: bulb.PowerLevel(),
	}
	payload := msg.Payload.(*implifx.LightSetPowerLanMessage)
	bulb.poweredOn = payload.Level == 0xffff

	winActionCh <- ui.PowerAction{
		On:       bulb.poweredOn,
		Duration: payload.Duration,
	}

	return writer(false, controlifx.LightStatePowerType, responsePayload)
}
