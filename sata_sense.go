package smart

import "fmt"

// Sense data layout constants
const (
	senseHeaderLen          = 8
	descHeaderLen           = 2
	descAtaReturnPayloadLen = 12

	// descAtaReturnExtendFlag is bit 0 of the ATA Status Return descriptor's extend
	// flags byte: set for 48-bit (LBA48) commands.
	descAtaReturnExtendFlag = 0x01

	// Minimum length of a fixed format sense buffer holding the ATA registers
	// (offsets 3..11) and the ASC/ASCQ signature (bytes 12/13) that
	// identifies an ATA PASS-THROUGH report.
	fixedSenseMinLen = 14
)

// A view over sense data in either fixed or descriptor format.
type ataSense []byte

func (s ataSense) ResponseCode() byte { return s[0] & _SCSI_SENSE_RESPONSE_CODE_MASK }

func (s ataSense) HasResponseCode() bool {
	return len(s) >= 1
}

func (s ataSense) IsFixedFormat() bool {
	return s.ResponseCode()&_SCSI_SENSE_FORMAT_MASK == _SCSI_SENSE_FIXED_FORMAT
}

func (s ataSense) IsDescriptorFormat() bool {
	return s.ResponseCode()&_SCSI_SENSE_FORMAT_MASK == _SCSI_SENSE_DESCRIPTOR_FORMAT
}

func (s ataSense) SenseKey() byte {
	if s.IsFixedFormat() {
		return ataSenseFixedFormat(s).SenseKey()
	}
	return ataSenseDescriptorFormat(s).SenseKey()
}

func (s ataSense) ASC() byte {
	if s.IsFixedFormat() {
		return ataSenseFixedFormat(s).ASC()
	}
	return ataSenseDescriptorFormat(s).ASC()
}

func (s ataSense) ASCQ() byte {
	if s.IsFixedFormat() {
		return ataSenseFixedFormat(s).ASCQ()
	}
	return ataSenseDescriptorFormat(s).ASCQ()
}

// IsAtaPassThroughInfo reports the RECOVERED_ERROR/00h,1Dh signature that a
// SAT layer emits for a successful CK_COND ATA PASS-THROUGH command ("ATA
// PASS THROUGH INFORMATION AVAILABLE").
func (s ataSense) IsAtaPassThroughInfo() bool {
	return s.SenseKey() == _SCSI_SK_RECOVERED_ERROR && s.ASC() == _SCSI_ASC_ATA_PT_INFO && s.ASCQ() == _SCSI_ASCQ_ATA_PT_INFO
}

func (s ataSense) IsValid() bool {
	switch {
	case s.IsDescriptorFormat():
		return ataSenseDescriptorFormat(s).IsValid()
	case s.IsFixedFormat():
		return ataSenseFixedFormat(s).IsValid()
	default:
		return false
	}
}

// Descriptor format sense buffer. Can contain multiple descriptors. Emitted by Linux libata for CK_COND ATA PASS-THROUGH commands.
type ataSenseDescriptorFormat []byte

func (s ataSenseDescriptorFormat) SenseKey() byte             { return s[1] }
func (s ataSenseDescriptorFormat) ASC() byte                  { return s[2] }
func (s ataSenseDescriptorFormat) ASCQ() byte                 { return s[3] }
func (s ataSenseDescriptorFormat) AdditionalSenseLength() int { return int(s[7]) }

func (s ataSenseDescriptorFormat) Payload() []byte {
	return s[senseHeaderLen : senseHeaderLen+s.AdditionalSenseLength()]
}

func (s ataSenseDescriptorFormat) IsValid() bool {
	return len(s) >= senseHeaderLen && len(s) >= senseHeaderLen+s.AdditionalSenseLength()
}

// 14-byte SAT ATA Status Return descriptor
type ataReturnDescriptor []byte

func (d ataReturnDescriptor) Extend() bool      { return d[2]&descAtaReturnExtendFlag != 0 }
func (d ataReturnDescriptor) Error() byte       { return d[3] }
func (d ataReturnDescriptor) SectorCount() byte { return d[5] }
func (d ataReturnDescriptor) LBALow() byte      { return d[7] }
func (d ataReturnDescriptor) LBAMid() byte      { return d[9] }
func (d ataReturnDescriptor) LBAHigh() byte     { return d[11] }
func (d ataReturnDescriptor) Device() byte      { return d[12] }
func (d ataReturnDescriptor) Status() byte      { return d[13] }

// Fixed format sense buffer carrying ATA registers. Emitted by some USB bridges.
type ataSenseFixedFormat []byte

func (f ataSenseFixedFormat) SenseKey() byte    { return f[2] & 0x0f }
func (f ataSenseFixedFormat) Error() byte       { return f[3] }
func (f ataSenseFixedFormat) Status() byte      { return f[4] }
func (f ataSenseFixedFormat) Device() byte      { return f[5] }
func (f ataSenseFixedFormat) SectorCount() byte { return f[6] }
func (f ataSenseFixedFormat) Extend() bool      { return f[8]&0x80 != 0 }
func (f ataSenseFixedFormat) LBALow() byte      { return f[9] }
func (f ataSenseFixedFormat) LBAMid() byte      { return f[10] }
func (f ataSenseFixedFormat) LBAHigh() byte     { return f[11] }
func (f ataSenseFixedFormat) ASC() byte         { return f[12] }
func (f ataSenseFixedFormat) ASCQ() byte        { return f[13] }

// IsValid reports whether the buffer is long enough for a fixed format sense
// carrying the ATA registers (offsets 3..11) and the ASC/ASCQ signature
// (bytes 12/13) required to identify an ATA PASS-THROUGH report.
func (f ataSenseFixedFormat) IsValid() bool {
	return len(f) >= fixedSenseMinLen
}

// findAtaReturnDescriptor walks the descriptor area of descriptor format sense
// data looking for the ATA Status Return descriptor. It returns ok=true only
// for a complete >= 14-byte descriptor.
func findAtaReturnDescriptor(sense ataSenseDescriptorFormat) (ataReturnDescriptor, bool) {
	if !sense.IsValid() {
		return nil, false
	}

	payload := sense.Payload()
	for offset := 0; offset < len(payload); {
		code := payload[offset]
		if offset+descHeaderLen > len(payload) {
			return nil, false
		}
		descPayloadLen := int(payload[offset+1])

		if offset+descHeaderLen+descPayloadLen > len(payload) {
			return nil, false
		}

		if code == _SCSI_SENSE_DESC_ATA_RETURN && descPayloadLen >= descAtaReturnPayloadLen {
			return ataReturnDescriptor(payload[offset : offset+descHeaderLen+descPayloadLen]), true
		}

		offset += descHeaderLen + descPayloadLen
	}

	return nil, false
}

// ataSenseStatus validates the sense data of a CK_COND ATA PASS-THROUGH
// command and extracts the ATA STATUS and ERROR registers.
// A successful command reports RECOVERED_ERROR/00h,1Dh; anything else is a failure.
func ataSenseStatus(sense []byte) (ataStatus, ataError byte, err error) {
	s := ataSense(sense)

	if !s.HasResponseCode() {
		return 0, 0, fmt.Errorf("sense buffer too short (%d bytes)", len(sense))
	}

	switch {
	case s.IsDescriptorFormat():
		desc := ataSenseDescriptorFormat(sense)
		if !desc.IsValid() {
			return 0, 0, fmt.Errorf("descriptor format sense buffer too short (%d bytes)", len(desc))
		}
		ata, ok := findAtaReturnDescriptor(desc)
		if s.IsAtaPassThroughInfo() {
			if !ok {
				return 0, 0, fmt.Errorf("no ATA Status Return descriptor in sense data")
			}
			return ata.Status(), ata.Error(), nil
		}
		if ok {
			return 0, 0, fmt.Errorf("command failed: sense key %#02x, ASC/ASCQ %#02x/%#02x, ATA status %#02x, error %#02x",
				desc.SenseKey(), desc.ASC(), desc.ASCQ(), ata.Status(), ata.Error())
		}
		return 0, 0, fmt.Errorf("unexpected sense key/ASC/ASCQ: %#02x/%#02x/%#02x", desc.SenseKey(), desc.ASC(), desc.ASCQ())
	case s.IsFixedFormat():
		fixed := ataSenseFixedFormat(sense)
		if !fixed.IsValid() {
			return 0, 0, fmt.Errorf("fixed format sense buffer too short (%d bytes)", len(fixed))
		}
		if s.IsAtaPassThroughInfo() {
			return fixed.Status(), fixed.Error(), nil
		}
		return 0, 0, fmt.Errorf("command failed: sense key %#02x, ASC/ASCQ %#02x/%#02x, ATA status %#02x, error %#02x",
			fixed.SenseKey(), fixed.ASC(), fixed.ASCQ(), fixed.Status(), fixed.Error())
	default:
		return 0, 0, fmt.Errorf("unsupported sense data format (response code %#02x)", sense[0])
	}
}
