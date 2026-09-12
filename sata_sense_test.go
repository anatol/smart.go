package smart

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// buildStatusReturnDescriptorSense builds descriptor format sense data as
// libata emits it for a successful CK_COND command: RECOVERED_ERROR/00h/1Dh
// plus an ATA Status Return descriptor carrying the registers.
func buildStatusReturnDescriptorSense(nsect byte) []byte {
	sense := make([]byte, senseHeaderLen+descHeaderLen+descAtaReturnPayloadLen)
	sense[0] = _SCSI_SENSE_DESCRIPTOR_FORMAT
	sense[1] = _SCSI_SK_RECOVERED_ERROR
	sense[2] = _SCSI_ASC_ATA_PT_INFO
	sense[3] = _SCSI_ASCQ_ATA_PT_INFO
	sense[7] = descHeaderLen + descAtaReturnPayloadLen // additional sense length

	desc := sense[senseHeaderLen:]
	desc[0] = _SCSI_SENSE_DESC_ATA_RETURN
	desc[1] = descAtaReturnPayloadLen // additional descriptor length (payload only)
	desc[5] = nsect                   // sector count register carries the power mode
	return sense
}

// buildFixedSense builds fixed format sense data as emitted by some USB SAT bridges
func buildFixedSense(nsect byte) []byte {
	// A full frame is 18 bytes: the 8-byte header plus 10 additional bytes.
	// Devices shall emit at least 18 bytes when the allocation length allows
	// (Seagate manual §3.38); shorter responses are legal but truncated.
	const (
		additionalSenseLen = 10
		fixedSenseLen      = senseHeaderLen + additionalSenseLen
	)

	sense := make([]byte, fixedSenseLen)
	sense[0] = _SCSI_SENSE_FIXED_FORMAT
	sense[2] = _SCSI_SK_RECOVERED_ERROR // sense key: low nibble of byte 2
	sense[3] = 0x00                     // error register
	sense[4] = 0x50                     // status register: DRDY+DSC
	sense[5] = 0x00                     // device register
	sense[6] = nsect                    // sector count register carries the power mode
	sense[7] = additionalSenseLen
	sense[12] = _SCSI_ASC_ATA_PT_INFO
	sense[13] = _SCSI_ASCQ_ATA_PT_INFO
	return sense
}

// buildStatusReturnDescriptorSenseFull is like buildStatusReturnDescriptorSense
// but fills all ATA registers of the return descriptor, for accessor and
// ataSenseStatus tests.
func buildStatusReturnDescriptorSenseFull(status, errReg, device, nsect, lbaLow, lbaMid, lbaHigh byte, extend bool) []byte {
	sense := buildStatusReturnDescriptorSense(nsect)
	desc := sense[senseHeaderLen:]
	if extend {
		desc[2] |= descAtaReturnExtendFlag
	}
	desc[3] = errReg
	desc[7] = lbaLow
	desc[9] = lbaMid
	desc[11] = lbaHigh
	desc[12] = device
	desc[13] = status
	return sense
}

func TestAtaReturnDescriptorAccessors(t *testing.T) {
	t.Parallel()

	sense := buildStatusReturnDescriptorSenseFull(0x50, 0x04, 0x40, 0x2a, 0x11, 0x22, 0x33, true)
	desc, ok := findAtaReturnDescriptor(sense)
	require.True(t, ok)
	require.Equal(t, byte(0x50), desc.Status())
	require.Equal(t, byte(0x04), desc.Error())
	require.Equal(t, byte(0x40), desc.Device())
	require.Equal(t, byte(0x2a), desc.SectorCount())
	require.Equal(t, byte(0x11), desc.LBALow())
	require.Equal(t, byte(0x22), desc.LBAMid())
	require.Equal(t, byte(0x33), desc.LBAHigh())
	require.True(t, desc.Extend())

	desc, ok = findAtaReturnDescriptor(buildStatusReturnDescriptorSenseFull(0x50, 0x00, 0x00, 0x00, 0, 0, 0, false))
	require.True(t, ok)
	require.False(t, desc.Extend())
}

func TestAtaSenseFixedFormatAccessors(t *testing.T) {
	t.Parallel()

	// Full 18-byte frame with all ATA registers populated.
	sense := buildFixedSense(0x2a)
	sense[3] = 0x04  // error register
	sense[5] = 0x40  // device register
	sense[8] = 0x80  // extend flag (bit 7)
	sense[9] = 0x11  // LBA low
	sense[10] = 0x22 // LBA mid
	sense[11] = 0x33 // LBA high

	fixed := ataSenseFixedFormat(sense)
	require.Equal(t, byte(_SCSI_SK_RECOVERED_ERROR), fixed.SenseKey())
	require.Equal(t, byte(0x04), fixed.Error())
	require.Equal(t, byte(0x50), fixed.Status())
	require.Equal(t, byte(0x40), fixed.Device())
	require.Equal(t, byte(0x2a), fixed.SectorCount())
	require.True(t, fixed.Extend())
	require.Equal(t, byte(0x11), fixed.LBALow())
	require.Equal(t, byte(0x22), fixed.LBAMid())
	require.Equal(t, byte(0x33), fixed.LBAHigh())
	require.Equal(t, byte(_SCSI_ASC_ATA_PT_INFO), fixed.ASC())
	require.Equal(t, byte(_SCSI_ASCQ_ATA_PT_INFO), fixed.ASCQ())

	// Sense key masks off the high nibble of byte 2.
	sense[2] = 0xf1
	require.Equal(t, byte(_SCSI_SK_RECOVERED_ERROR), fixed.SenseKey())

	// Extend flag is bit 7 only: bits 6/5 (nonzero upper bytes) must not set it.
	sense[8] = 0x60
	require.False(t, fixed.Extend())

	// Without the extend flag.
	sense[8] = 0x00
	require.False(t, fixed.Extend())
}

func TestAtaSenseFixedFormatValid(t *testing.T) {
	t.Parallel()

	// Minimum (14 bytes): registers (offsets 3..11) and the ASC/ASCQ
	// signature (bytes 12/13) are all readable.
	require.True(t, ataSenseFixedFormat(make([]byte, fixedSenseMinLen)).IsValid())

	// Too short: the signature would be missing or partial.
	require.False(t, ataSenseFixedFormat(make([]byte, fixedSenseMinLen-1)).IsValid())
	require.False(t, ataSenseFixedFormat(nil).IsValid())
}

func TestAtaSenseDescriptorFormatAccessors(t *testing.T) {
	t.Parallel()

	sense := buildStatusReturnDescriptorSense(0xff)
	desc := ataSenseDescriptorFormat(sense)
	require.Equal(t, byte(_SCSI_SK_RECOVERED_ERROR), desc.SenseKey())
	require.Equal(t, byte(_SCSI_ASC_ATA_PT_INFO), desc.ASC())
	require.Equal(t, byte(_SCSI_ASCQ_ATA_PT_INFO), desc.ASCQ())

	// Payload is exactly the declared descriptor area: 14 bytes holding the
	// ATA Status Return descriptor.
	payload := desc.Payload()
	require.Len(t, payload, descHeaderLen+12)
	require.Equal(t, byte(_SCSI_SENSE_DESC_ATA_RETURN), payload[0])
	require.Equal(t, byte(12), payload[1])
	require.Equal(t, byte(0xff), ataReturnDescriptor(payload[0:]).SectorCount())

	// Payload reflects a changed ADDITIONAL SENSE LENGTH.
	sense[7] = 0
	require.Empty(t, desc.Payload())
}

func TestAtaSenseDescriptorFormatValid(t *testing.T) {
	t.Parallel()

	// Complete declared sense data: header + descriptor area.
	sense := buildStatusReturnDescriptorSense(0xff)
	require.True(t, ataSenseDescriptorFormat(sense).IsValid())

	// Truncated descriptor area (declared length exceeds the buffer).
	sense[7] = 40
	require.False(t, ataSenseDescriptorFormat(sense).IsValid())

	// Exactly the 8-byte header with an empty descriptor area.
	sense = make([]byte, senseHeaderLen)
	require.True(t, ataSenseDescriptorFormat(sense).IsValid())

	// Shorter than the header.
	require.False(t, ataSenseDescriptorFormat(make([]byte, senseHeaderLen-1)).IsValid())
}

func TestAtaSense(t *testing.T) {
	t.Parallel()

	// Descriptor format: signature and getters.
	desc := buildStatusReturnDescriptorSense(0xff)
	s := ataSense(desc)
	require.True(t, s.IsDescriptorFormat())
	require.False(t, s.IsFixedFormat())
	require.Equal(t, byte(_SCSI_SENSE_DESCRIPTOR_FORMAT), s.ResponseCode())
	require.Equal(t, byte(_SCSI_SK_RECOVERED_ERROR), s.SenseKey())
	require.Equal(t, byte(_SCSI_ASC_ATA_PT_INFO), s.ASC())
	require.Equal(t, byte(_SCSI_ASCQ_ATA_PT_INFO), s.ASCQ())
	require.True(t, s.IsAtaPassThroughInfo())

	// Fixed format: signature and getters.
	fixed := buildFixedSense(0x00)
	s = ataSense(fixed)
	require.True(t, s.IsFixedFormat())
	require.False(t, s.IsDescriptorFormat())
	require.Equal(t, byte(_SCSI_SENSE_FIXED_FORMAT), s.ResponseCode())
	require.Equal(t, byte(_SCSI_SK_RECOVERED_ERROR), s.SenseKey())
	require.Equal(t, byte(_SCSI_ASC_ATA_PT_INFO), s.ASC())
	require.Equal(t, byte(_SCSI_ASCQ_ATA_PT_INFO), s.ASCQ())
	require.True(t, s.IsAtaPassThroughInfo())

	// Deferred error variants (71h/73h) map onto the same formats.
	require.True(t, ataSense([]byte{0x71}).IsFixedFormat())
	require.True(t, ataSense([]byte{0x73}).IsDescriptorFormat())

	// Mismatching signature.
	fixed[2] = _SCSI_SK_ILLEGAL_REQUEST
	require.False(t, s.IsAtaPassThroughInfo())
}

func TestFindAtaReturnDescriptorRejectsShort(t *testing.T) {
	t.Parallel()

	// Descriptor with payload < 12 bytes must not be returned: the accessors
	// are only safe on complete descriptors.
	sense := make([]byte, senseHeaderLen+descHeaderLen+6)
	sense[0] = _SCSI_SENSE_DESCRIPTOR_FORMAT
	sense[1] = _SCSI_SK_RECOVERED_ERROR
	sense[2] = _SCSI_ASC_ATA_PT_INFO
	sense[3] = _SCSI_ASCQ_ATA_PT_INFO
	sense[7] = descHeaderLen + 6
	sense[8] = _SCSI_SENSE_DESC_ATA_RETURN
	sense[9] = 6
	_, ok := findAtaReturnDescriptor(sense)
	require.False(t, ok)
}

func TestAtaSenseStatusSuccess(t *testing.T) {
	t.Parallel()

	// Descriptor format success signature.
	status, errReg, err := ataSenseStatus(buildStatusReturnDescriptorSenseFull(0x50, 0x00, 0x40, 0x00, 0, 0, 0, false))
	require.NoError(t, err)
	require.Equal(t, byte(0x50), status)
	require.Equal(t, byte(0x00), errReg)

	// Fixed format success signature.
	sense := buildFixedSense(0x00)
	status, errReg, err = ataSenseStatus(sense)
	require.NoError(t, err)
	require.Equal(t, byte(0x50), status)
	require.Equal(t, byte(0x00), errReg)
}

func TestAtaSenseStatusFailures(t *testing.T) {
	t.Parallel()

	// Descriptor format: ILLEGAL REQUEST sense carrying an ATA return
	// descriptor with ERR status + ABRT error (device rejected the command).
	sense := buildStatusReturnDescriptorSenseFull(_ATA_STATUS_ERR, _ATA_ERROR_ABORT, 0x40, 0x00, 0, 0, 0, false)
	sense[1] = _SCSI_SK_ILLEGAL_REQUEST
	sense[2] = _SCSI_ASC_ILLEGAL_FIELD
	sense[3] = 0x00
	_, _, err := ataSenseStatus(sense)
	require.Error(t, err)
	require.ErrorContains(t, err, "ATA status 0x01")
	require.ErrorContains(t, err, "error 0x04")

	// Descriptor format: error sense without an ATA return descriptor.
	sense = make([]byte, senseHeaderLen)
	sense[0] = _SCSI_SENSE_DESCRIPTOR_FORMAT
	sense[1] = _SCSI_SK_ILLEGAL_REQUEST
	sense[2] = _SCSI_ASC_ILLEGAL_FIELD
	sense[3] = 0x00
	_, _, err = ataSenseStatus(sense)
	require.Error(t, err)
	require.ErrorContains(t, err, "unexpected sense key/ASC/ASCQ")

	// Fixed format: error sense (sense key in byte 2 low nibble).
	sense = buildFixedSense(0x00)
	sense[2] = _SCSI_SK_ILLEGAL_REQUEST
	sense[3] = _ATA_ERROR_ABORT
	sense[4] = _ATA_STATUS_ERR
	sense[12] = 0x00
	sense[13] = 0x00
	_, _, err = ataSenseStatus(sense)
	require.Error(t, err)
	require.ErrorContains(t, err, "ATA status 0x01")
	require.ErrorContains(t, err, "error 0x04")

	// Fixed format buffer too short to reach ASC/ASCQ at bytes 12/13.
	_, _, err = ataSenseStatus([]byte{_SCSI_SENSE_FIXED_FORMAT, 0x00, _SCSI_SK_RECOVERED_ERROR, 0x00, 0x50, 0x00, 0x00, 0x0a, 0, 0, 0, 0})
	require.Error(t, err)

	// Too short buffer.
	_, _, err = ataSenseStatus([]byte{_SCSI_SENSE_DESCRIPTOR_FORMAT, _SCSI_SK_RECOVERED_ERROR})
	require.Error(t, err)

	// Unsupported response code.
	sense = make([]byte, senseHeaderLen)
	sense[0] = 0x74
	_, _, err = ataSenseStatus(sense)
	require.Error(t, err)
}
