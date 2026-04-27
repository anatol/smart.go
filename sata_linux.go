package smart

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

func OpenSata(name string) (*SataDevice, error) {
	fd, err := unix.Open(name, unix.O_RDONLY, 0o600)
	if err != nil {
		return nil, err
	}

	i, err := scsiInquiry(fd)
	if err != nil {
		unix.Close(fd)
		return nil, err
	}

	// Check if this is a direct access block device (Peripheral & 0x1f == 0)
	// This is required for SAT (SCSI ATA Translation) devices
	deviceType := i.Peripheral & 0x1f
	if deviceType != 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("not a direct access block device (type=%d)", deviceType)
	}

	// Try to detect if this is a SATA device
	// Standard SAT devices report "ATA     " in VendorIdent
	// However, some USB bridges don't properly implement this and report
	// the actual drive vendor instead. In such cases, we try ATA IDENTIFY
	// to see if the device responds.
	isLikelySAT := bytes.Equal(i.VendorIdent[:], []byte(_SATA_IDENT))

	dev := SataDevice{fd: fd}

	// For standard SAT identifiers, proceed directly
	// For other direct access devices, try ATA IDENTIFY to see if it's a SAT device
	if !isLikelySAT {
		// Try ATA IDENTIFY to test if this device supports ATA commands
		// This handles USB bridges that don't report "ATA     " properly
		respBuf := make([]byte, 512)
		cdb := cdb16{_SCSI_ATA_PASSTHRU_16}
		cdb[1] = 0x08                  // ATA protocol: bits [4:1] = 4 (PIO data-in), bit 0 = 0 (multiple commands)
		cdb[2] = 0x0e                  // BYT_BLOK=1 (transfer length in 512-byte blocks), T_LENGTH=2 (from sector count field), T_DIR=1 (from device)
		cdb[14] = _ATA_IDENTIFY_DEVICE // command

		// If ATA IDENTIFY succeeds, this is a SAT device
		if err := scsiSendCdb(fd, cdb[:], respBuf); err != nil {
			unix.Close(fd)
			return nil, fmt.Errorf("device does not respond to ATA IDENTIFY (not a SATA/SAT device): %w", err)
		}
	}

	id, err := dev.Identify()
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	mapping, bug, err := findAttributesMapping(id.ModelNumber(), id.FirmwareRevision())
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	dev.attributeMapping = mapping
	dev.firmwareBug = bug

	return &dev, nil
}

func (d *SataDevice) Close() error {
	return unix.Close(d.fd)
}

func (d *SataDevice) Identify() (*AtaIdentifyDevice, error) {
	var resp AtaIdentifyDevice

	respBuf := make([]byte, 512)

	cdb := cdb16{_SCSI_ATA_PASSTHRU_16}
	cdb[1] = 0x08                  // ATA protocol: bits [4:1] = 4 (PIO data-in), bit 0 = 0 (multiple commands)
	cdb[2] = 0x0e                  // BYT_BLOK=1 (transfer length in 512-byte blocks), T_LENGTH=2 (from sector count field), T_DIR=1 (from device)
	cdb[14] = _ATA_IDENTIFY_DEVICE // command

	if err := scsiSendCdb(d.fd, cdb[:], respBuf); err != nil {
		return nil, fmt.Errorf("sendCDB ATA IDENTIFY: %w", err)
	}

	if err := binary.Read(bytes.NewBuffer(respBuf), binary.LittleEndian, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (d *SataDevice) readSMARTLog(logPage uint8) ([]byte, error) {
	respBuf := make([]byte, 512)

	cdb := cdb16{_SCSI_ATA_PASSTHRU_16}
	cdb[1] = 0x08            // ATA protocol (4 << 1, PIO data-in)
	cdb[2] = 0x0e            // BYT_BLOK = 1, T_LENGTH = 2, T_DIR = 1
	cdb[4] = _SMART_READ_LOG // feature LSB
	cdb[6] = 0x01            // sector count: transfer 1 sector (512 bytes)
	cdb[8] = logPage         // SMART log page number (maps to LBA low byte)
	cdb[10] = 0x4f           // LBA mid: SMART magic signature byte (0x4F)
	cdb[12] = 0xc2           // LBA high: SMART magic signature byte (0xC2); together 0x4FC2 identifies a SMART command
	cdb[14] = _ATA_SMART     // command

	if err := scsiSendCdb(d.fd, cdb[:], respBuf); err != nil {
		return nil, fmt.Errorf("scsiSendCdb SMART READ LOG: %w", err)
	}

	return respBuf, nil
}

func (d *SataDevice) readSMARTData() (*AtaSmartPageRaw, error) {
	cdb := cdb16{_SCSI_ATA_PASSTHRU_16}
	cdb[1] = 0x08             // ATA protocol (4 << 1, PIO data-in)
	cdb[2] = 0x0e             // BYT_BLOK = 1, T_LENGTH = 2, T_DIR = 1
	cdb[4] = _SMART_READ_DATA // feature LSB
	cdb[10] = 0x4f            // LBA mid: SMART magic signature byte (0x4F)
	cdb[12] = 0xc2            // LBA high: SMART magic signature byte (0xC2)
	cdb[14] = _ATA_SMART      // command

	respBuf := make([]byte, 512)

	if err := scsiSendCdb(d.fd, cdb[:], respBuf); err != nil {
		return nil, fmt.Errorf("scsiSendCdb SMART READ DATA: %w", err)
	}

	page := AtaSmartPageRaw{}
	// 362 = 2 bytes version + 30 attributes × 12 bytes each
	if err := binary.Read(bytes.NewBuffer(respBuf[:362]), binary.LittleEndian, &page); err != nil {
		return nil, err
	}

	return &page, nil
}

func (d *SataDevice) ReadSMARTLogDirectory() (*AtaSmartLogDirectory, error) {
	buf, err := d.readSMARTLog(0x00)
	if err != nil {
		return nil, err
	}

	dir := AtaSmartLogDirectory{}
	if err := binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &dir); err != nil {
		return nil, err
	}

	return &dir, nil
}

func (d *SataDevice) ReadSMARTErrorLogSummary() (*AtaSmartErrorLogSummary, error) {
	buf, err := d.readSMARTLog(0x01)
	if err != nil {
		return nil, err
	}

	summary := AtaSmartErrorLogSummary{}
	if err := binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &summary); err != nil {
		return nil, err
	}

	return &summary, nil
}

func (d *SataDevice) ReadSMARTSelfTestLog() (*AtaSmartSelfTestLog, error) {
	buf, err := d.readSMARTLog(0x06)
	if err != nil {
		return nil, err
	}

	log := AtaSmartSelfTestLog{}
	if err := binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &log); err != nil {
		return nil, err
	}

	return &log, nil
}

func (d *SataDevice) readSMARTThresholds() (*AtaSmartThresholdsPageRaw, error) {
	cdb := cdb16{_SCSI_ATA_PASSTHRU_16}
	cdb[1] = 0x08                   // ATA protocol (4 << 1, PIO data-in)
	cdb[2] = 0x0e                   // BYT_BLOK = 1, T_LENGTH = 2, T_DIR = 1
	cdb[4] = _SMART_READ_THRESHOLDS // feature LSB
	cdb[8] = 0x1                    // LBA low: required to be 1 for READ THRESHOLDS
	cdb[10] = 0x4f                  // LBA mid: SMART magic signature byte (0x4F)
	cdb[12] = 0xc2                  // LBA high: SMART magic signature byte (0xC2)
	cdb[14] = _ATA_SMART            // command

	respBuf := make([]byte, 512)

	if err := scsiSendCdb(d.fd, cdb[:], respBuf); err != nil {
		return nil, fmt.Errorf("scsiSendCdb SMART READ THRESHOLD: %w", err)
	}

	if !checksum(respBuf) {
		return nil, fmt.Errorf("invalid checksum for SMART THRESHOLD data")
	}

	page := AtaSmartThresholdsPageRaw{}
	if err := binary.Read(bytes.NewBuffer(respBuf[:]), binary.LittleEndian, &page); err != nil {
		return nil, err
	}

	return &page, nil
}
