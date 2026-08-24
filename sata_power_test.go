package smart

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var ataPowerModes = map[byte]AtaPowerMode{
	0x00: AtaPowerModeStandby,
	0x01: AtaPowerModeStandbyY,
	0x80: AtaPowerModeIdle,
	0x81: AtaPowerModeIdleA,
	0x82: AtaPowerModeIdleB,
	0x83: AtaPowerModeIdleC,
	0xff: AtaPowerModeActiveOrIdle,
}

// buildDescriptorSense builds descriptor format sense data as libata emits it
// for a successful CK_COND command: RECOVERED_ERROR/00h/1Dh plus an ATA
// Status Return descriptor carrying the registers.
func buildDescriptorSense(nsect byte) []byte {
	// ATA Status Return descriptor: 2-byte header + 12-byte register payload.
	const ataReturnPayloadLen = 12

	sense := make([]byte, senseHeaderLen+descHeaderLen+ataReturnPayloadLen)
	sense[0] = _SCSI_SENSE_DESCRIPTOR_FORMAT
	sense[1] = _SCSI_SK_RECOVERED_ERROR
	sense[2] = _SCSI_ASC_ATA_PT_INFO
	sense[3] = _SCSI_ASCQ_ATA_PT_INFO
	sense[7] = descHeaderLen + ataReturnPayloadLen // additional sense length

	desc := sense[8:]
	desc[0] = _SCSI_SENSE_DESC_ATA_RETURN
	desc[1] = ataReturnPayloadLen // additional descriptor length (payload only)
	desc[5] = nsect               // sector count register carries the power mode
	return sense
}

func TestAtaPowerModeFromSenseDescriptorFormat(t *testing.T) {
	t.Parallel()

	for nsect, expected := range ataPowerModes {
		mode, err := ataPowerModeFromSense(buildDescriptorSense(nsect))
		require.NoError(t, err)
		require.Equal(t, expected, mode)
	}
}

// buildFixedSense builds fixed format sense data as emitted by some USB SAT bridges.
func buildFixedSense(nsect byte) []byte {
	// Fixed format sense data as emitted by some USB SAT bridges. A full
	// frame is 18 bytes: the 8-byte header plus up to 10 additional bytes
	// (ADDITIONAL SENSE LENGTH is capped at 10); SCSI advises hosts to
	// allocate at least 18 bytes so the device never truncates the response.
	const (
		additionalSenseLen = 10
		fixedSenseLen      = senseHeaderLen + additionalSenseLen
	)

	sense := make([]byte, fixedSenseLen)
	sense[0] = _SCSI_SENSE_FIXED_FORMAT
	sense[1] = _SCSI_SK_RECOVERED_ERROR
	sense[2] = _SCSI_ASC_ATA_PT_INFO
	sense[3] = 0x00  // error register
	sense[4] = 0x50  // status register: DRDY+DSC
	sense[5] = 0x00  // device register
	sense[6] = nsect // sector count register carries the power mode
	sense[7] = additionalSenseLen
	return sense
}

func TestAtaPowerModeFromSenseFixedFormat(t *testing.T) {
	t.Parallel()

	for nsect, expected := range ataPowerModes {
		mode, err := ataPowerModeFromSense(buildFixedSense(nsect))
		require.NoError(t, err)
		require.Equal(t, expected, mode)
	}
}

// buildMultiDescriptorSense prepends a non-ATA Information descriptor before
// the ATA return descriptor, forcing the parser to skip by full descriptor size.
func buildMultiDescriptorSense(nsect byte) []byte {
	const firstDescPayloadLen = 10
	const secondDescPayloadLen = 12
	const firstDescLen = descHeaderLen + firstDescPayloadLen
	const secondDescLen = descHeaderLen + secondDescPayloadLen

	sense := make([]byte, senseHeaderLen+firstDescLen+secondDescLen)
	sense[0] = _SCSI_SENSE_DESCRIPTOR_FORMAT
	sense[1] = _SCSI_SK_RECOVERED_ERROR
	sense[2] = _SCSI_ASC_ATA_PT_INFO
	sense[3] = _SCSI_ASCQ_ATA_PT_INFO
	sense[7] = firstDescLen + secondDescLen // additional sense length

	// Information descriptor.
	firstDesc := sense[senseHeaderLen : senseHeaderLen+firstDescLen]
	firstDesc[0] = 0x00
	firstDesc[1] = firstDescPayloadLen

	// ATA Status Return descriptor.
	secondDesc := sense[senseHeaderLen+firstDescLen:]
	secondDesc[0] = _SCSI_SENSE_DESC_ATA_RETURN
	secondDesc[1] = secondDescPayloadLen
	secondDesc[5] = nsect
	return sense
}

func TestAtaPowerModeFromSenseMultipleDescriptors(t *testing.T) {
	t.Parallel()

	mode, err := ataPowerModeFromSense(buildMultiDescriptorSense(0xff))
	require.NoError(t, err)
	require.Equal(t, AtaPowerModeActiveOrIdle, mode)

	mode, err = ataPowerModeFromSense(buildMultiDescriptorSense(0x00))
	require.NoError(t, err)
	require.Equal(t, AtaPowerModeStandby, mode)
}

func TestAtaPowerModeFromSenseErrors(t *testing.T) {
	t.Parallel()

	// Too short buffer
	_, err := ataPowerModeFromSense([]byte{_SCSI_SENSE_DESCRIPTOR_FORMAT, _SCSI_SK_RECOVERED_ERROR})
	require.Error(t, err)

	// Unsupported response code
	sense := make([]byte, senseHeaderLen)
	sense[0] = 0x74
	_, err = ataPowerModeFromSense(sense)
	require.Error(t, err)

	// Truncated descriptor format sense data
	sense = buildDescriptorSense(0xff)
	sense[7] = 40 // claims more data than the buffer holds
	_, err = ataPowerModeFromSense(sense)
	require.Error(t, err)

	// Descriptor format without an ATA Status Return descriptor: one other
	// descriptor filling the declared descriptor area exactly; its payload
	// stays zeroed by make and must be skipped without being read.
	sense = make([]byte, senseHeaderLen+descHeaderLen+2)
	sense[0] = _SCSI_SENSE_DESCRIPTOR_FORMAT
	sense[1] = _SCSI_SK_RECOVERED_ERROR
	sense[2] = _SCSI_ASC_ATA_PT_INFO
	sense[3] = _SCSI_ASCQ_ATA_PT_INFO
	sense[7] = descHeaderLen + 2 // additional sense length
	other := sense[8:]
	other[0] = 0x00 // some other descriptor code
	other[1] = 2    // payload-only length
	_, err = ataPowerModeFromSense(sense)
	require.Error(t, err)

	// Descriptor area ending with a stray byte: the last descriptor header is
	// truncated (odd additional sense length).
	sense = make([]byte, senseHeaderLen+1)
	sense[0] = _SCSI_SENSE_DESCRIPTOR_FORMAT
	sense[1] = _SCSI_SK_RECOVERED_ERROR
	sense[2] = _SCSI_ASC_ATA_PT_INFO
	sense[3] = _SCSI_ASCQ_ATA_PT_INFO
	sense[7] = 1                           // additional sense length: one stray byte follows
	sense[8] = _SCSI_SENSE_DESC_ATA_RETURN // descriptor code without its length byte
	_, err = ataPowerModeFromSense(sense)
	require.Error(t, err)

	// Unexpected sector count value
	_, err = ataPowerModeFromSense(buildDescriptorSense(0x42))
	require.Error(t, err)

	// Error sense carrying stale ATA registers (libata reports them even when
	// rejecting a bad CDB) must be rejected, not parsed as a power mode.
	sense = buildDescriptorSense(0x00) // stale nsect=0 would read as "standby"
	sense[1] = _SCSI_SK_ILLEGAL_REQUEST
	sense[2] = _SCSI_ASC_ILLEGAL_FIELD
	sense[3] = 0x00
	_, err = ataPowerModeFromSense(sense)
	require.Error(t, err)

	// Fixed format buffer too short to hold the registers
	_, err = ataPowerModeFromSense([]byte{_SCSI_SENSE_FIXED_FORMAT, _SCSI_SK_RECOVERED_ERROR})
	require.Error(t, err)
}
