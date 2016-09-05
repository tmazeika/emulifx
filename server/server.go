package server

import (
	"encoding"
	"github.com/bionicrm/emulifx/ui"
	"gopkg.in/lifx-tools/controlifx.v1"
	"gopkg.in/lifx-tools/implifx.v1"
	"log"
	"net"
	"time"
)

type writer func(always bool, t uint16, msg encoding.BinaryMarshaler) error

var (
	bulb struct {
		service             int8
		port                uint16
		time                int64
		resetSwitchPosition uint8
		dummyLoadOn         bool
		hostInfo            struct {
			signal         float32
			tx             uint32
			rx             uint32
			mcuTemperature uint16
		}
		hostFirmware struct {
			build   int64
			install int64
			version uint32
		}
		wifiInfo struct {
			signal         float32
			tx             uint32
			rx             uint32
			mcuTemperature int16
		}
		wifiFirmware struct {
			build   int64
			install int64
			version uint32
		}
		powerLevel uint16
		label      string
		tags       struct {
			tags  int64
			label string
		}
		version struct {
			vendor  uint32
			product uint32
			version uint32
		}
		info struct {
			time     int64
			uptime   int64
			downtime int64
		}
		mcuRailVoltage  uint32
		factoryTestMode struct {
			on       bool
			disabled bool
		}
		site     [6]byte
		location struct {
			location  [16]byte
			label     string
			updatedAt int64
		}
		group struct {
			group     [16]byte
			label     string
			updatedAt int64
		}
		owner struct {
			owner     [16]byte
			label     string
			updatedAt int64
		}
		state struct {
			color controlifx.HSBK
			dim   int16
			label string
			tags  uint64
		}
		lightRailVoltage  uint32
		lightTemperature  int16
		lightSimpleEvents []struct {
			time     int64
			power    uint16
			duration uint32
			waveform int8
			max      uint16
		}
		wanStatus  int8
		wanAuthKey [32]byte
		wanHost    struct {
			host               string
			insecureSkipVerify bool
		}
		wifi struct {
			networkInterface int8
			status           int8
		}
		wifiAccessPoints struct {
			networkInterface int8
			ssid             string
			security         int8
			strength         int16
			channel          uint16
		}
		wifiAccessPoint struct {
			networkInterface int8
			ssid             string
			pass             string
			security         int8
		}
		sensorAmbientLightLux float32
		sensorDimmerVoltage   uint32

		// Extra.
		startTime int64
	}

	winStopCh   = make(chan interface{})
	winActionCh = make(chan interface{})
)

func Start(addr string, hasColor bool) error {
	defer func() {
		winStopCh <- 0
	}()

	// Connect.
	conn, err := connect(addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Mock MAC.
	conn.Mac = 0xd0738f86bfaf

	configureBulb(conn.Port(), hasColor)

	var windowClosed bool

	go func() {
		if err := ui.ShowWindow(hasColor, conn.LocalAddr().String(), winStopCh, winActionCh); err != nil {
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

		bulb.wifiInfo.rx += uint32(n)

		if err := handle(recMsg, func(always bool, t uint16, payload encoding.BinaryMarshaler) error {
			tx, err := conn.Respond(always, raddr, recMsg, t, payload)
			bulb.wifiInfo.tx += uint32(tx)

			return err
		}); err != nil {
			log.Println(err)
		}
	}
}

func connect(addr string) (conn implifx.Connection, err error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return conn, err
	}

	return implifx.ListenOnOtherPort(host, portStr)
}

func configureBulb(port uint16, hasColor bool) {
	bulb.service = controlifx.UdpService
	bulb.port = port

	// Mock HostFirmware.
	bulb.hostFirmware.build = 1467178139000000000
	bulb.hostFirmware.version = 1968197120

	// Mock WifiInfo.
	bulb.wifiInfo.signal = 1e-5

	// Mock WifiFirmware.
	bulb.wifiFirmware.build = 1456093684000000000

	if hasColor {
		bulb.version.vendor = controlifx.Color1000VendorId
		bulb.version.product = controlifx.Color1000ProductId
	} else {
		bulb.version.vendor = controlifx.White800HighVVendorId
		bulb.version.product = controlifx.White800HighVProductId
	}

	bulb.state.color.Kelvin = 3500

	// Extra.
	bulb.startTime = time.Now().UnixNano()
}

func handle(msg implifx.ReceivableLanMessage, w writer) error {
	switch msg.Header.ProtocolHeader.Type {
	case controlifx.GetServiceType:
		return getService(w)
	case controlifx.GetHostInfoType:
		return getHostInfo(w)
	case controlifx.GetHostFirmwareType:
		return getHostFirmware(w)
	case controlifx.GetWifiInfoType:
		return getWifiInfo(w)
	case controlifx.GetWifiFirmwareType:
		return getWifiFirmware(w)
	case controlifx.GetPowerType:
		return getPower(w)
	case controlifx.SetPowerType:
		return setPower(msg, w)
	case controlifx.GetLabelType:
		return getLabel(w)
	case controlifx.SetLabelType:
		return setLabel(msg, w)
	case controlifx.GetVersionType:
		return getVersion(w)
	case controlifx.GetInfoType:
		return getInfo(w)
	case controlifx.GetLocationType:
		return getLocation(w)
	case controlifx.GetGroupType:
		return getGroup(w)
	case controlifx.GetOwnerType:
		return getOwner(w)
	case controlifx.SetOwnerType:
		return setOwner(msg, w)
	case controlifx.EchoRequestType:
		return echoRequest(msg, w)
	case controlifx.LightGetType:
		return lightGet(w)
	case controlifx.LightSetColorType:
		return lightSetColor(msg, w)
	case controlifx.LightGetPowerType:
		return lightGetPower(w)
	case controlifx.LightSetPowerType:
		return lightSetPower(msg, w)
	}

	return nil
}

func getService(w writer) error {
	return w(true, controlifx.StateServiceType, &implifx.StateServiceLanMessage{
		Service: controlifx.UdpService,
		Port:    uint32(bulb.port),
	})
}

func getHostInfo(w writer) error {
	return w(true, controlifx.StateHostInfoType, &implifx.StateHostInfoLanMessage{})
}

func getHostFirmware(w writer) error {
	return w(true, controlifx.StateHostFirmwareType, &implifx.StateHostFirmwareLanMessage{
		Build:   uint64(bulb.hostFirmware.build),
		Version: bulb.hostFirmware.version,
	})
}

func getWifiInfo(w writer) error {
	return w(true, controlifx.StateWifiInfoType, &implifx.StateWifiInfoLanMessage{
		Signal: bulb.wifiInfo.signal,
		Tx:     bulb.wifiInfo.tx,
		Rx:     bulb.wifiInfo.rx,
	})
}

func getWifiFirmware(w writer) error {
	return w(true, controlifx.StateWifiFirmwareType, &implifx.StateWifiFirmwareLanMessage{
		Build:   uint64(bulb.wifiFirmware.build),
		Version: bulb.wifiFirmware.version,
	})
}

func getPower(w writer) error {
	return w(true, controlifx.StatePowerType, &implifx.StatePowerLanMessage{
		Level: bulb.powerLevel,
	})
}

func setPower(msg implifx.ReceivableLanMessage, w writer) error {
	responsePayload := &implifx.StatePowerLanMessage{
		Level: bulb.powerLevel,
	}
	bulb.powerLevel = msg.Payload.(*implifx.SetPowerLanMessage).Level

	winActionCh <- ui.PowerAction{
		On: bulb.powerLevel == 0xffff,
	}

	return w(false, controlifx.StatePowerType, responsePayload)
}

func getLabel(w writer) error {
	return w(true, controlifx.StateLabelType, &implifx.StateLabelLanMessage{
		Label: bulb.label,
	})
}

func setLabel(msg implifx.ReceivableLanMessage, w writer) error {
	bulb.label = msg.Payload.(*implifx.SetLabelLanMessage).Label

	return w(false, controlifx.StateLabelType, &implifx.StateLabelLanMessage{
		Label: bulb.label,
	})
}

func getVersion(w writer) error {
	return w(true, controlifx.StateVersionType, &implifx.StateVersionLanMessage{
		Vendor:  bulb.version.vendor,
		Product: bulb.version.product,
		Version: bulb.version.version,
	})
}

func getInfo(w writer) error {
	now := time.Now().UnixNano()

	return w(true, controlifx.StateInfoType, &implifx.StateInfoLanMessage{
		Time:     uint64(now),
		Uptime:   uint64(now - bulb.startTime),
		Downtime: 0,
	})
}

func getLocation(w writer) error {
	return w(true, controlifx.StateLocationType, &implifx.StateLocationLanMessage{
		Location:  bulb.location.location,
		Label:     bulb.location.label,
		UpdatedAt: uint64(bulb.location.updatedAt),
	})
}

func getGroup(w writer) error {
	return w(true, controlifx.StateGroupType, &implifx.StateGroupLanMessage{
		Group:     bulb.group.group,
		Label:     bulb.group.label,
		UpdatedAt: uint64(bulb.group.updatedAt),
	})
}

func getOwner(w writer) error {
	return w(true, controlifx.StateOwnerType, &implifx.StateOwnerLanMessage{
		Owner:     bulb.owner.owner,
		Label:     bulb.owner.label,
		UpdatedAt: uint64(bulb.owner.updatedAt),
	})
}

func setOwner(msg implifx.ReceivableLanMessage, w writer) error {
	payload := msg.Payload.(*implifx.SetOwnerLanMessage)
	bulb.owner.owner = payload.Owner
	bulb.owner.label = payload.Label
	bulb.owner.updatedAt = time.Now().UnixNano()

	return w(false, controlifx.StateOwnerType, &implifx.StateOwnerLanMessage{
		Owner:     bulb.owner.owner,
		Label:     bulb.owner.label,
		UpdatedAt: uint64(bulb.owner.updatedAt),
	})
}

func echoRequest(msg implifx.ReceivableLanMessage, w writer) error {
	return w(true, controlifx.EchoResponseType, &implifx.EchoResponseLanMessage{
		Payload: msg.Payload.(*implifx.EchoRequestLanMessage).Payload,
	})
}

func lightGet(w writer) error {
	return w(true, controlifx.LightStateType, &implifx.LightStateLanMessage{
		Color: bulb.state.color,
		Power: bulb.powerLevel,
		Label: bulb.label,
	})
}

func lightSetColor(msg implifx.ReceivableLanMessage, w writer) error {
	responsePayload := &implifx.LightStateLanMessage{
		Color: bulb.state.color,
		Power: bulb.powerLevel,
		Label: bulb.label,
	}
	payload := msg.Payload.(*implifx.LightSetColorLanMessage)
	bulb.state.color = payload.Color

	winActionCh <- ui.ColorAction{
		Color:    payload.Color,
		Duration: payload.Duration,
	}

	return w(false, controlifx.LightStateType, responsePayload)
}

func lightGetPower(w writer) error {
	return w(true, controlifx.LightStatePowerType, &implifx.LightStatePowerLanMessage{
		Level: bulb.powerLevel,
	})
}

func lightSetPower(msg implifx.ReceivableLanMessage, w writer) error {
	responsePayload := &implifx.StatePowerLanMessage{
		Level: bulb.powerLevel,
	}
	payload := msg.Payload.(*implifx.LightSetPowerLanMessage)
	bulb.powerLevel = payload.Level

	winActionCh <- ui.PowerAction{
		On:       bulb.powerLevel == 0xffff,
		Duration: payload.Duration,
	}

	return w(false, controlifx.LightStatePowerType, responsePayload)
}
