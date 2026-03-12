package smart

import (
	"errors"
	"fmt"
)

// ErrOSUnsupported is returned on unsupported operating systems.
var ErrOSUnsupported = errors.New("os not supported")

type GenericAttributes struct {
	// Temperature represents the device temperature in Celsius
	Temperature uint64
	// Read represents a number of data units (LBA) read
	Read uint64
	// Written represents a number of data units (LBA) written
	Written uint64
	// PowerOnHours represents a power on time in hours
	PowerOnHours uint64
	// PowerCycles represents the number of power cycles
	PowerCycles uint64
}

type Device interface {
	Type() string
	Close() error

	// ReadGenericAttributes is an *experimental* API for quick access to the most common device SMART properties
	// This API as well as content of GenericAttributes is subject for a change.
	ReadGenericAttributes() (*GenericAttributes, error)
}

func Open(path string) (Device, error) {
	n, nvmeErr := OpenNVMe(path)
	if nvmeErr == nil {
		_, _, err := n.Identify()
		if err == nil {
			return n, nil
		}
		n.Close()
		nvmeErr = fmt.Errorf("nvme identify: %w", err)
	}

	a, sataErr := OpenSata(path)
	if sataErr == nil {
		return a, nil
	}

	s, scsiErr := OpenScsi(path)
	if scsiErr == nil {
		return s, nil
	}

	return nil, errors.Join(nvmeErr, sataErr, scsiErr)
}
