package model

import "slices"

// DeviceType is the device class of an event.
//
// It maps onto the LowCardinality(String) device column, which keeps a dictionary of
// distinct values in every part. The set is therefore closed: an unrecognised value
// becomes DeviceUnknown instead of a new dictionary entry, because a client is free to
// send any string and a dictionary is not free to grow without cost.
type DeviceType string

// The supported device classes (PLAN.md 5.1).
const (
	DeviceDesktop DeviceType = "desktop"
	DeviceMobile  DeviceType = "mobile"
	DeviceTablet  DeviceType = "tablet"
	DeviceBot     DeviceType = "bot"
	DeviceUnknown DeviceType = "unknown"
)

// deviceTypes drives both the check and anything that enumerates the set, so a new class
// can never be accepted by Valid while a caller listing the options still omits it.
var deviceTypes = []DeviceType{
	DeviceDesktop, DeviceMobile, DeviceTablet, DeviceBot, DeviceUnknown,
}

// Valid reports whether d is a device class this build understands.
func (d DeviceType) Valid() bool { return slices.Contains(deviceTypes, d) }

// DeviceTypes returns the closed set of device classes, in contract order. The result is a
// copy: the caller cannot reshape the vocabulary by writing to it.
func DeviceTypes() []DeviceType { return slices.Clone(deviceTypes) }
