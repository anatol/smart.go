package smart

import (
	"testing"
	"time"

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

func TestAtaPowerModeFromSenseDescriptorFormat(t *testing.T) {
	t.Parallel()

	for nsect, expected := range ataPowerModes {
		mode, err := ataPowerModeFromSense(buildStatusReturnDescriptorSense(nsect))
		require.NoError(t, err)
		require.Equal(t, expected, mode)
	}
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
	sense = buildStatusReturnDescriptorSense(0xff)
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
	_, err = ataPowerModeFromSense(buildStatusReturnDescriptorSense(0x42))
	require.Error(t, err)

	// Error sense carrying stale ATA registers (libata reports them even when
	// rejecting a bad CDB) must be rejected, not parsed as a power mode.
	sense = buildStatusReturnDescriptorSense(0x00) // stale nsect=0 would read as "standby"
	sense[1] = _SCSI_SK_ILLEGAL_REQUEST
	sense[2] = _SCSI_ASC_ILLEGAL_FIELD
	sense[3] = 0x00
	_, err = ataPowerModeFromSense(sense)
	require.Error(t, err)

	// Fixed format buffer too short to hold the registers
	_, err = ataPowerModeFromSense([]byte{_SCSI_SENSE_FIXED_FORMAT, _SCSI_SK_RECOVERED_ERROR})
	require.Error(t, err)
}

func TestAtaPowerModeFromSenseFixedFormatSignatureGuard(t *testing.T) {
	t.Parallel()

	// Fixed format error sense carrying stale registers must be rejected,
	// not parsed as a power mode (mirrors the descriptor format guard).
	sense := buildFixedSense(0x00) // stale nsect=0 would read as "standby"
	sense[2] = _SCSI_SK_ILLEGAL_REQUEST
	sense[12] = _SCSI_ASC_ILLEGAL_FIELD
	sense[13] = 0x00
	_, err := ataPowerModeFromSense(sense)
	require.Error(t, err)
	require.ErrorContains(t, err, "unexpected sense key/ASC/ASCQ")

	// A short fixed format response (truncated tail: the transferred amount
	// is bounded by the initiator's allocation length) carries no or a
	// partial ASC/ASCQ signature and must be rejected: without it the
	// registers cannot be authenticated as an ATA pass-through report.
	short := make([]byte, fixedSenseMinLen-1)
	short[0] = _SCSI_SENSE_FIXED_FORMAT
	short[6] = 0xff
	_, err = ataPowerModeFromSense(short)
	require.Error(t, err)
	require.ErrorContains(t, err, "fixed format sense buffer is invalid")
}

func TestEncodeAtaSpinDownTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d    time.Duration
		want byte
	}{
		{0, 0},
		{5 * time.Second, 1},
		{10 * time.Second, 2},
		{20 * time.Minute, 240},
		{30 * time.Minute, 241},
		{5*time.Hour + 30*time.Minute, 251},
	}
	for _, test := range tests {
		got, err := encodeAtaSpinDownTimeout(test.d)
		require.NoErrorf(t, err, "encode %s", test.d)
		require.Equalf(t, test.want, got, "encode %s", test.d)
	}

	// Round-trip
	for _, test := range tests {
		v, err := encodeAtaSpinDownTimeout(test.d)
		require.NoError(t, err)
		require.Equalf(t, test.d, decodeAtaSpinDownTimeout(v), "round-trip %s", test.d)
	}

	// Rejections: not representable exactly
	for _, d := range []time.Duration{
		1 * time.Second,
		7 * time.Second,
		25 * time.Minute,
		21 * time.Minute,
		6 * time.Hour,
		-5 * time.Second,
	} {
		_, err := encodeAtaSpinDownTimeout(d)
		require.Errorf(t, err, "encode %s must fail", d)
	}
}

func TestDecodeAtaSpinDownTimeout(t *testing.T) {
	t.Parallel()

	require.Equal(t, time.Duration(0), decodeAtaSpinDownTimeout(0))
	require.Equal(t, 5*time.Second, decodeAtaSpinDownTimeout(1))
	require.Equal(t, 20*time.Minute, decodeAtaSpinDownTimeout(240))
	require.Equal(t, 30*time.Minute, decodeAtaSpinDownTimeout(241))
	require.Equal(t, 5*time.Hour+30*time.Minute, decodeAtaSpinDownTimeout(251))
	require.Equal(t, 21*time.Minute, decodeAtaSpinDownTimeout(252))
	require.Equal(t, 21*time.Minute+15*time.Second, decodeAtaSpinDownTimeout(255))
	// Vendor-specific and reserved values decode to zero.
	require.Equal(t, time.Duration(0), decodeAtaSpinDownTimeout(253))
	require.Equal(t, time.Duration(0), decodeAtaSpinDownTimeout(254))
}
