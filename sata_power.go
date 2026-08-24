package smart

import "fmt"

// ATA power management commands
const _ATA_CHECK_POWER_MODE = 0xe5

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

// Sense data layout constants
const (
	senseHeaderLen = 8
	descHeaderLen  = 2

	// Minimum length of a fixed format sense buffer carrying the ATA
	// registers (offsets 3..11). Responses shorter than the full 18-byte
	// frame are legal: the transferred amount is bounded by the initiator's
	// allocation length, and ADDITIONAL SENSE LENGTH (byte 7) keeps the
	// data self-describing.
	fixedSenseMinLen = 12
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
// from the SCSI sense data returned by a SAT layer. With CK_COND set, the ATA
// registers arrive either in descriptor format sense data (Linux libata) or
// fixed format sense data (some USB bridges). The power state is carried in
// the sector count register (ACS-3 Table 204).
// Note: Some USB bridges do not implement CK_COND and reject such commands.
func ataPowerModeFromSense(sense []byte) (AtaPowerMode, error) {
	if len(sense) < senseHeaderLen {
		return 0, fmt.Errorf("sense buffer too short (%d bytes)", len(sense))
	}

	// A successful CK_COND command reports RECOVERED_ERROR/00h,1Dh; anything
	// else means the command failed and the descriptor may carry stale
	// registers that must not be mistaken for a power mode.
	if sense[0]&_SCSI_SENSE_RESPONSE_CODE_MASK == _SCSI_SENSE_DESCRIPTOR_FORMAT {
		if sense[1] != _SCSI_SK_RECOVERED_ERROR || sense[2] != _SCSI_ASC_ATA_PT_INFO || sense[3] != _SCSI_ASCQ_ATA_PT_INFO {
			return 0, fmt.Errorf("unexpected sense key/ASC/ASCQ: %#02x/%#02x/%#02x", sense[1], sense[2], sense[3])
		}
	}

	var nsect byte
	var found bool

	switch sense[0] & _SCSI_SENSE_RESPONSE_CODE_MASK {
	case _SCSI_SENSE_DESCRIPTOR_FORMAT:
		additionalLen := int(sense[7])
		totalSenseLen := senseHeaderLen + additionalLen

		if len(sense) < totalSenseLen {
			return 0, fmt.Errorf("truncated descriptor format sense data: %d bytes, expected %d", len(sense), totalSenseLen)
		}

		for offset := senseHeaderLen; offset < totalSenseLen; {
			code := sense[offset]
			if offset+descHeaderLen > totalSenseLen {
				return 0, fmt.Errorf("truncated sense descriptor header at offset %d (code %#02x)", offset, code)
			}
			descPayloadLen := int(sense[offset+1])

			if offset+descHeaderLen+descPayloadLen > totalSenseLen {
				return 0, fmt.Errorf("truncated sense descriptor at offset %d (code %#02x, payload length %d)", offset, code, descPayloadLen)
			}

			if code == _SCSI_SENSE_DESC_ATA_RETURN && descPayloadLen >= 6 {
				nsect = sense[offset+5] // sector count register
				found = true
				break
			}

			offset += descHeaderLen + descPayloadLen
		}
	case _SCSI_SENSE_FIXED_FORMAT:
		// Fixed register offsets: [3]=error, [4]=status, [5]=device,
		// [6]=sector count, [9..11]=lba low/mid/high.
		if len(sense) < fixedSenseMinLen {
			return 0, fmt.Errorf("fixed format sense buffer too short (%d bytes)", len(sense))
		}
		nsect = sense[6]
		found = true
	default:
		return 0, fmt.Errorf("unsupported sense data format (response code %#02x)", sense[0])
	}

	if !found {
		return 0, fmt.Errorf("no ATA Status Return descriptor in sense data")
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
