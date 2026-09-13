package smart

import (
	"fmt"
	"time"
)

// ataNonDataCdb builds a 16-byte CDB for a non-data ATA PASS-THROUGH(16)
// command with CK_COND set.
func ataNonDataCdb(cmd byte) cdb16 {
	cdb := cdb16{_SCSI_ATA_PASSTHRU_16}
	cdb[1] = 0x06 // ATA protocol: non-data
	cdb[2] = 0x20 // CK_COND=1: report ATA registers via sense data
	cdb[14] = cmd
	return cdb
}

// ataSendNonData issues a non-data ATA command and validates the sense data.
func (d *SataDevice) ataSendNonData(cmd byte) error {
	cdb := ataNonDataCdb(cmd)
	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return err
	}
	_, _, err = ataSenseStatus(sense)
	return err
}

// CheckPowerMode reports the drive's power state. The command never spins up a spun-down drive.
func (d *SataDevice) CheckPowerMode() (AtaPowerMode, error) {
	cdb := ataNonDataCdb(_ATA_CHECK_POWER_MODE)
	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return 0, fmt.Errorf("scsiSendCdbSense CHECK POWER MODE: %w", err)
	}
	mode, err := ataPowerModeFromSense(sense)
	if err != nil {
		return 0, fmt.Errorf("CHECK POWER MODE: %w", err)
	}
	return mode, nil
}

// SetPowerMode transitions the drive into the given power mode. Idle and AtaPowerModeActiveOrIdle
// cannot be forced by any ATA command (the drive enters it on I/O alone).
func (d *SataDevice) SetPowerMode(mode AtaPowerMode) error {
	switch mode {
	case AtaPowerModeIdle:
		return d.Idle()
	case AtaPowerModeStandby:
		return d.Standby()
	}
	cond, ok := powerConditionForMode(mode)
	if !ok {
		return fmt.Errorf("power mode %s cannot be set by command (active is entered by I/O only)", mode)
	}
	return d.setPowerCondition(cond)
}

func (d *SataDevice) setPowerCondition(cond byte) error {
	cdb := ataNonDataCdb(_ATA_SET_FEATURES)
	cdb[4] = _ATA_SETFEATURES_EPC
	cdb[6] = cond
	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return err
	}
	_, _, err = ataSenseStatus(sense)
	return err
}

// SetStandbyTimer sets the Standby timer to the given duration and spins the
// drive down to Standby_z. 0 disables the timer.
func (d *SataDevice) SetStandbyTimer(timeout time.Duration) error {
	count, err := encodeAtaSpinDownTimeout(timeout)
	if err != nil {
		return err
	}
	return d.setSpinDownTimer(count)
}

// SetStandbyTimerRaw passes a raw timer value to the STANDBY command
// without validation. Also spins the drive down to Standby_z.
func (d *SataDevice) SetStandbyTimerRaw(raw byte) error {
	return d.setSpinDownTimer(raw)
}

func (d *SataDevice) setSpinDownTimer(count byte) error {
	cdb := ataNonDataCdb(_ATA_STANDBY)
	cdb[6] = count
	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return err
	}
	_, _, err = ataSenseStatus(sense)
	return err
}

// SetIdleTimer sets the Standby timer to the given duration via the IDLE
// command (which also enters Idle_a) without spinning the drive down.
func (d *SataDevice) SetIdleTimer(timeout time.Duration) error {
	count, err := encodeAtaSpinDownTimeout(timeout)
	if err != nil {
		return err
	}
	return d.setIdleTimerRaw(count)
}

// SetIdleTimerRaw passes a raw timer value to the IDLE command
// without validation. Also enters Idle_a.
func (d *SataDevice) SetIdleTimerRaw(raw byte) error {
	return d.setIdleTimerRaw(raw)
}

func (d *SataDevice) setIdleTimerRaw(count byte) error {
	cdb := ataNonDataCdb(_ATA_IDLE)
	cdb[6] = count
	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return err
	}
	_, _, err = ataSenseStatus(sense)
	return err
}

// SetAPMLevel enables the APM feature set and sets the power management level
// (1 = minimum power consumption, 254 = maximum performance; ACS-3 Table 106).
// APM and EPC are mutually exclusive (ACS-3 4.9.4): enabling APM disables EPC.
func (d *SataDevice) SetAPMLevel(level uint8) error {
	if level < 1 || level > 254 {
		return fmt.Errorf("APM level %d out of range (1..254)", level)
	}
	cdb := ataNonDataCdb(_ATA_SET_FEATURES)
	cdb[4] = _ATA_SETFEATURES_EN_APM
	cdb[6] = level
	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return err
	}
	_, _, err = ataSenseStatus(sense)
	return err
}

// DisableAPM disables the APM feature set (SET FEATURES subcommand 0x85).
func (d *SataDevice) DisableAPM() error {
	cdb := ataNonDataCdb(_ATA_SET_FEATURES)
	cdb[4] = _ATA_SETFEATURES_DIS_APM
	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return err
	}
	_, _, err = ataSenseStatus(sense)
	return err
}

// IdleUnload issues IDLE IMMEDIATE with the Unload feature: the heads retract
// to the ramp/landing zone and the drive enters Idle_a. Platters keep
// spinning; the next media access reloads the heads.
func (d *SataDevice) IdleUnload() error {
	cdb := ataNonDataCdb(_ATA_IDLE_IMMEDIATE)
	cdb[4] = _ATA_IDLE_UNLOAD_FEATURE
	cdb[7] = _ATA_IDLE_UNLOAD_LBA_LOW
	cdb[9] = _ATA_IDLE_UNLOAD_LBA_MID
	cdb[11] = _ATA_IDLE_UNLOAD_LBA_HGH
	sense, err := scsiSendCdbSense(d.fd, cdb[:], nil)
	if err != nil {
		return err
	}
	_, _, err = ataSenseStatus(sense)
	return err
}

// The drive enters PM3:Sleep state. No ATA command exits PM3:Sleep - recovery
// requires a hardware/software reset (e.g. sysfs rescan, replug, or reboot).
func (d *SataDevice) Sleep() error {
	return d.ataSendNonData(_ATA_SLEEP)
}

// Platters are spun down
func (d *SataDevice) Standby() error {
	return d.ataSendNonData(_ATA_STANDBY_IMMEDIATE)
}

// The drive responds to commands without a spin-up delay
func (d *SataDevice) Idle() error {
	return d.ataSendNonData(_ATA_IDLE_IMMEDIATE)
}
