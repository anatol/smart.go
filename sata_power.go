package smart

import (
	"fmt"
	"time"
)

// ATA power management commands
const (
	_ATA_CHECK_POWER_MODE  = 0xe5
	_ATA_STANDBY_IMMEDIATE = 0xe0
	_ATA_IDLE_IMMEDIATE    = 0xe1
	_ATA_STANDBY           = 0xe2
	_ATA_IDLE              = 0xe3
	_ATA_SLEEP             = 0xe6
	_ATA_SET_FEATURES      = 0xef
)

// SET FEATURES subcommands (feature register values)
const (
	_ATA_SETFEATURES_EN_APM  = 0x05 // APM level in the count register
	_ATA_SETFEATURES_DIS_APM = 0x85
	_ATA_SETFEATURES_EPC     = 0x4a // Extended Power Conditions subcommand
)

// SET FEATURES EPC subcommand codes (LBA field bits 3:0)
const (
	_ATA_EPC_SUBCMD_GO_TO_POWER_CONDITION = 0x1
)

// SET FEATURES power condition IDs (count register, ACS-3 Table 112)
const (
	_ATA_POWER_CONDITION_STANDBY_Z = 0x00
	_ATA_POWER_CONDITION_STANDBY_Y = 0x01
	_ATA_POWER_CONDITION_IDLE_A    = 0x81
	_ATA_POWER_CONDITION_IDLE_B    = 0x82
	_ATA_POWER_CONDITION_IDLE_C    = 0x83
)

// IDLE IMMEDIATE with Unload feature (ACS-3 Table 54): feature 0x44,
// count 0x00, LBA 055_4E4Ch ('L','N','U' in lba low/mid/high).
const (
	_ATA_IDLE_UNLOAD_FEATURE = 0x44
	_ATA_IDLE_UNLOAD_LBA_LOW = 0x4c
	_ATA_IDLE_UNLOAD_LBA_MID = 0x4e
	_ATA_IDLE_UNLOAD_LBA_HGH = 0x55
)

// ATA status register bits (ACS-3 Table 18)
const (
	_ATA_STATUS_BUSY = 1 << 7
	_ATA_STATUS_DRDY = 1 << 6
	_ATA_STATUS_DF   = 1 << 5
	_ATA_STATUS_DSC  = 1 << 4
	_ATA_STATUS_DRQ  = 1 << 3
	_ATA_STATUS_ERR  = 1 << 0
)

// ATA error register bits (ACS-3 Table 19)
const (
	_ATA_ERROR_ABORT = 1 << 2
)

// AtaPowerMode represents the power state of an ATA drive as reported by the
// CHECK POWER MODE command.
type AtaPowerMode uint8

const (
	// AtaPowerModeStandby means the drive is in the PM2:Standby state
	// (either the EPC feature set is not enabled, or the device is in the
	// Standby_z power condition). Platters are spun down.
	AtaPowerModeStandby AtaPowerMode = 0x00

	// AtaPowerModeStandbyY means the drive is in the PM2:Standby state,
	// Standby_y power condition (EPC enabled). Platters are spun down;
	// recovery to Active is faster than from Standby_z.
	AtaPowerModeStandbyY AtaPowerMode = 0x01

	// AtaPowerModeIdle means the drive is in the PM1:Idle state with the EPC
	// feature set unsupported or disabled. The drive responds to commands
	// without a spin-up delay.
	AtaPowerModeIdle AtaPowerMode = 0x80

	// AtaPowerModeIdleA means the drive is in the PM1:Idle state, Idle_a
	// Power condition (EPC enabled): the lowest-latency idle substate.
	AtaPowerModeIdleA AtaPowerMode = 0x81

	// AtaPowerModeIdleB means the drive is in the PM1:Idle state, Idle_b
	// power condition (EPC enabled): intermediate power saving; many drives
	// unload their heads in this condition (vendor-specific).
	AtaPowerModeIdleB AtaPowerMode = 0x82

	// AtaPowerModeIdleC means the drive is in the PM1:Idle state, Idle_c
	// power condition (EPC enabled): the deepest Idle substate; many drives
	// unload heads and reduce spindle RPM (vendor-specific).
	AtaPowerModeIdleC AtaPowerMode = 0x83

	// AtaPowerModeActiveOrIdle means the drive is in the PM0:Active state or
	// the PM1:Idle state. Platters are spinning.
	AtaPowerModeActiveOrIdle AtaPowerMode = 0xff
)

func (m AtaPowerMode) String() string {
	switch m {
	case AtaPowerModeStandby:
		return "standby"
	case AtaPowerModeStandbyY:
		return "standby_y"
	case AtaPowerModeIdle:
		return "idle"
	case AtaPowerModeIdleA:
		return "idle_a"
	case AtaPowerModeIdleB:
		return "idle_b"
	case AtaPowerModeIdleC:
		return "idle_c"
	case AtaPowerModeActiveOrIdle:
		return "active or idle"
	default:
		return fmt.Sprintf("unknown (sector count %#02x)", uint8(m))
	}
}

// ataPowerModeFromSense extracts the power mode reported by CHECK POWER MODE
// from the SCSI sense data returned by a SAT layer. The power state is carried
// in the sector count register (ACS-3 Table 204).
// Note: Some USB bridges do not implement CK_COND and reject such commands.
func ataPowerModeFromSense(sense []byte) (AtaPowerMode, error) {
	s := ataSense(sense)

	if !s.HasResponseCode() {
		return 0, fmt.Errorf("sense buffer too short (%d bytes)", len(sense))
	}

	var nsect byte

	switch {
	case s.IsDescriptorFormat():
		if !ataSenseDescriptorFormat(sense).IsValid() {
			return 0, fmt.Errorf("descriptor format sense buffer is inavalid (length: %d bytes)", len(sense))
		}
		if !s.IsAtaPassThroughInfo() {
			return 0, fmt.Errorf("unexpected sense key/ASC/ASCQ: %#02x/%#02x/%#02x", s.SenseKey(), s.ASC(), s.ASCQ())
		}
		desc, ok := findAtaReturnDescriptor(sense)
		if !ok {
			return 0, fmt.Errorf("no ATA Status Return descriptor in sense data")
		}
		nsect = desc.SectorCount()
	case s.IsFixedFormat():
		fixed := ataSenseFixedFormat(sense)
		if !fixed.IsValid() {
			return 0, fmt.Errorf("fixed format sense buffer is invalid (length: %d bytes)", len(fixed))
		}
		if !s.IsAtaPassThroughInfo() {
			return 0, fmt.Errorf("unexpected sense key/ASC/ASCQ: %#02x/%#02x/%#02x", s.SenseKey(), s.ASC(), s.ASCQ())
		}
		nsect = fixed.SectorCount()
	default:
		return 0, fmt.Errorf("unsupported sense data format (response code %#02x)", s.ResponseCode())
	}

	switch powerMode := AtaPowerMode(nsect); powerMode {
	case AtaPowerModeStandby, AtaPowerModeStandbyY,
		AtaPowerModeIdle, AtaPowerModeIdleA, AtaPowerModeIdleB, AtaPowerModeIdleC,
		AtaPowerModeActiveOrIdle:
		return powerMode, nil
	default:
		return 0, fmt.Errorf("unexpected sector count value in CHECK POWER MODE response: %#02x", nsect)
	}
}

// encodeAtaSpinDownTimeout encodes d into the STANDBY/IDLE timer value
// (ACS-3 Table 52). Only exactly representable durations are accepted:
// 0, multiples of 5s up to 20m, or multiples of 30m up to 5h30m.
func encodeAtaSpinDownTimeout(d time.Duration) (byte, error) {
	switch {
	case d == 0:
		return 0, nil
	case d < 0:
		return 0, fmt.Errorf("duration %s cannot be negative", d)
	case d <= 20*time.Minute && d%(5*time.Second) == 0:
		return byte(d / (5 * time.Second)), nil
	case d <= 5*time.Hour+30*time.Minute && d%(30*time.Minute) == 0:
		return byte(d/(30*time.Minute)) + 240, nil
	default:
		return 0, fmt.Errorf("duration %s cannot be encoded as an ATA standby timer value (use 0, multiples of 5s up to 20m, or multiples of 30m up to 5h30m)", d)
	}
}

// decodeAtaSpinDownTimeout decodes a STANDBY/IDLE timer value into a duration
// (ACS-3 Table 52). Vendor-specific (0xfd) and reserved (0xfe) values return
// a zero duration.
func decodeAtaSpinDownTimeout(v byte) time.Duration {
	switch {
	case v == 0:
		return 0
	case v <= 240:
		return time.Duration(v) * 5 * time.Second
	case v <= 251:
		return time.Duration(v-240) * 30 * time.Minute
	case v == 252:
		return 21 * time.Minute
	case v == 255:
		return 21*time.Minute + 15*time.Second
	default: // 253 vendor-specific, 254 reserved
		return 0
	}
}

// powerConditionForMode maps an AtaPowerMode to the SET FEATURES power
// condition ID that selects it. Only the EPC-only conditions (Standby_y,
// Idle_a/b/c) are reachable this way; ok=false for the rest.
func powerConditionForMode(m AtaPowerMode) (cond byte, ok bool) {
	switch m {
	case AtaPowerModeStandbyY:
		return _ATA_POWER_CONDITION_STANDBY_Y, true
	case AtaPowerModeIdleA:
		return _ATA_POWER_CONDITION_IDLE_A, true
	case AtaPowerModeIdleB:
		return _ATA_POWER_CONDITION_IDLE_B, true
	case AtaPowerModeIdleC:
		return _ATA_POWER_CONDITION_IDLE_C, true
	default:
		return 0, false
	}
}
