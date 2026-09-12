package smart

import "errors"

// https://www.seagate.com/files/staticfiles/support/docs/manual/Interface%20manuals/100293068j.pdf

type ScsiDevice struct {
	fd int
}

func (d *ScsiDevice) Type() string {
	return "scsi"
}

const (
	_SG_IO = 0x2285

	_SG_INFO_OK_MASK = 0x1
	_SG_INFO_OK      = 0x0 /* no sense, host nor driver "noise" */
	_SG_INFO_CHECK   = 0x1 /* something abnormal happened */

	// Timeout in milliseconds
	_DEFAULT_TIMEOUT = 20000

	// SCSI status codes used by this package
	_SAM_STAT_CHECK_CONDITION = 0x02

	// SCSI commands used by this package
	_SCSI_INQUIRY          = 0x12
	_SCSI_MODE_SENSE_6     = 0x1a
	_SCSI_READ_CAPACITY_10 = 0x25

	_SG_DXFER_NONE        = -1
	_SG_DXFER_TO_DEV      = -2
	_SG_DXFER_FROM_DEV    = -3
	_SG_DXFER_TO_FROM_DEV = -4

	// Maximum sense data size for SG_IO. The sg driver copies out at most
	// min(mx_sb_len, SCSI_SENSE_BUFFERSIZE) bytes of sense data, where
	// SCSI_SENSE_BUFFERSIZE is the kernel-internal auto-sense buffer size
	// (96 bytes, include/scsi/scsi_device.h).
	_scsiSenseBufSize = 96
)

// SCSI sense constants for decoding SAT ATA PASS-THROUGH (CK_COND) register reports
const (
	// Mask applied to sense byte 0 (the RESPONSE CODE) to strip the VALID
	// bit. The remaining 7 bits encode the sense data format.
	_SCSI_SENSE_RESPONSE_CODE_MASK = 0x7f

	// Like _SCSI_SENSE_RESPONSE_CODE_MASK, but also strips bit 1, which says
	// whether the sense data answers the current command (70h/72h) or a
	// deferred one (71h/73h). Masking with 0x7e makes both cases compare
	// equal to the format constants below.
	_SCSI_SENSE_FORMAT_MASK = 0x7e

	// Sense data formats (response code, lower 7 bits)
	_SCSI_SENSE_FIXED_FORMAT      = 0x70
	_SCSI_SENSE_DESCRIPTOR_FORMAT = 0x72

	_SCSI_SK_RECOVERED_ERROR = 0x01
	_SCSI_ASC_ATA_PT_INFO    = 0x00
	_SCSI_ASCQ_ATA_PT_INFO   = 0x1d

	_SCSI_SK_ILLEGAL_REQUEST = 0x05
	_SCSI_ASC_ILLEGAL_FIELD  = 0x24

	_SCSI_SENSE_DESC_ATA_RETURN = 0x09
)

type ScsiInquiry struct {
	Peripheral   uint8 // peripheral qualifier + device type
	Rmb          uint8
	Version      uint8
	Flags        uint8
	RespLength   uint8
	Flags2       [3]uint8
	VendorIdent  [8]byte // if "ATA     " then it is ATA device
	ProductIdent [16]byte
	ProductRev   [4]byte
}

func (d *ScsiDevice) ReadGenericAttributes() (*GenericAttributes, error) {
	return nil, errors.ErrUnsupported
}
