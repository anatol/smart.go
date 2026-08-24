package smart

import "fmt"

// CheckPowerMode reports the drive's power state. The command never spins up a spun-down drive.
func (d *SataDevice) CheckPowerMode() (AtaPowerMode, error) {
	cdb := cdb16{_SCSI_ATA_PASSTHRU_16}
	cdb[1] = 0x06                   // ATA protocol: non-data
	cdb[2] = 0x20                   // CK_COND=1: report ATA registers via sense data
	cdb[14] = _ATA_CHECK_POWER_MODE // command

	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return 0, fmt.Errorf("scsiSendCdbSense CHECK POWER MODE: %w", err)
	}

	return ataPowerModeFromSense(sense)
}
