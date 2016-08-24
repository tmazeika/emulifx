package server

import (
	"github.com/bionicrm/controlifx"
	"encoding"
	"encoding/binary"
	"bytes"
	"fmt"
	"math"
)

type (
	sendableLanMessage      encoding.BinaryMarshaler
	receivableLanMessage    controlifx.ReceivableLanMessage

	setPowerLanMessage      controlifx.SetPowerLanMessage
	setLabelLanMessage      controlifx.SetLabelLanMessage
	echoRequestLanMessage   controlifx.EchoRequestLanMessage
	lightSetColorLanMessage controlifx.LightSetColorLanMessage
	lightSetPowerLanMessage controlifx.LightSetPowerLanMessage

	stateServiceLanMessage  controlifx.StateServiceLanMessage
	stateHostInfoLanMessage controlifx.StateHostInfoLanMessage
)

func (o *receivableLanMessage) UnmarshalBinary(data []byte) error {
	// Header.
	o.Header = controlifx.LanHeader{}
	if err := o.Header.UnmarshalBinary(data[:controlifx.LanHeaderSize]); err != nil {
		return err
	}

	// Payload.
	payload, err := getReceivablePayloadOfType(o.Header.ProtocolHeader.Type)
	if err != nil {
		return err
	}
	if payload == nil {
		return nil
	}

	o.Payload = payload

	return o.Payload.UnmarshalBinary(data[controlifx.LanHeaderSize:])
}

func getReceivablePayloadOfType(t uint16) (encoding.BinaryUnmarshaler, error) {
	var payload encoding.BinaryUnmarshaler

	switch t {
	case controlifx.SetPowerType:
		payload = &setPowerLanMessage{}
	case controlifx.SetLabelType:
		payload = &setLabelLanMessage{}
	case controlifx.EchoRequestType:
		payload = &echoRequestLanMessage{}
	case controlifx.LightSetColorType:
		payload = &lightSetColorLanMessage{}
	case controlifx.LightSetPowerType:
		payload = &lightSetPowerLanMessage{}
	case controlifx.GetServiceType:
		fallthrough
	case controlifx.GetHostInfoType:
		fallthrough
	case controlifx.GetHostFirmwareType:
		fallthrough
	case controlifx.GetWifiInfoType:
		fallthrough
	case controlifx.GetWifiFirmwareType:
		fallthrough
	case controlifx.GetPowerType:
		fallthrough
	case controlifx.GetLabelType:
		fallthrough
	case controlifx.GetVersionType:
		fallthrough
	case controlifx.GetInfoType:
		fallthrough
	case controlifx.GetLocationType:
		fallthrough
	case controlifx.GetGroupType:
		fallthrough
	case controlifx.LightGetType:
		fallthrough
	case controlifx.LightGetPowerType:
		return nil, nil
	default:
		return nil, fmt.Errorf("cannot create new payload of type %d; is it binary decodable?", t)
	}

	return payload, nil
}

func (o *setPowerLanMessage) UnmarshalBinary(data []byte) error {
	o.Level = controlifx.PowerLevel(binary.LittleEndian.Uint16(data[:2]))

	return nil
}

func (o *setLabelLanMessage) UnmarshalBinary(data []byte) error {
	o.Label = controlifx.Label(bytes.TrimRight(data[:32], "\x00"))

	return nil
}

func (o *echoRequestLanMessage) UnmarshalBinary(data []byte) error {
	copy(o.Payload[:], data[:64])

	return nil
}

func (o *lightSetColorLanMessage) UnmarshalBinary(data []byte) error {
	if err := o.Color.UnmarshalBinary(data[1:9]); err != nil {
		return err
	}

	o.Duration = binary.LittleEndian.Uint32(data[9:13])

	return nil
}

func (o *lightSetPowerLanMessage) UnmarshalBinary(data []byte) error {
	o.Level = controlifx.PowerLevel(binary.LittleEndian.Uint16(data[:2]))
	o.Duration = binary.LittleEndian.Uint32(data[2:6])

	return nil
}

func (o stateServiceLanMessage) MarshalBinary() (data []byte, _ error) {
	data = make([]byte, 5)

	// Service.
	data[0] = byte(o.Service)

	// Port.
	binary.LittleEndian.PutUint32(data[1:], o.Port)

	return data, nil
}

func (o stateHostInfoLanMessage) MarshalBinary() (data []byte, _ error) {
	data = make([]byte, 12)

	// Signal.
	binary.LittleEndian.PutUint32(data[:4], math.Float32bits(o.Signal))

	// Tx.
	binary.LittleEndian.PutUint32(data[4:8], o.Tx)

	// Rx.
	binary.LittleEndian.PutUint32(data[8:12], o.Rx)

	return data, nil
}
