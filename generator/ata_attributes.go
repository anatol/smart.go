// go:build ignore

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var outputFile = flag.String("output", "ata_device_database.go", "output file")

func main() {
	flag.Parse()

	if err := process(); err != nil {
		panic(err)
	}
}

func process() error {
	db, err := extractDatabase()
	if err != nil {
		return err
	}

	return printDeviceDatabase(db)
}

func extractDatabase() ([]byte, error) {
	d := os.TempDir()
	defer os.RemoveAll(d)

	cmd := exec.Command("curl", "https://raw.githubusercontent.com/mirror/smartmontools/master/drivedb.h")
	db, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	content := `
   #include <stdio.h>

   struct drive {
    const char * f1;
    const char * f2;
    const char * f3;
    const char * f4;
    const char * f5;
  };
  const struct drive drives[] = {
` +
		string(db) +
		`};

  int main(void) {
    int len = sizeof(drives) / sizeof(drives[0]);
    for (int i = 0; i < len; i++) {
      printf("%s\t\t\naa\t", drives[i].f1);
      printf("%s\t\t\naa\t", drives[i].f2);
      printf("%s\t\t\naa\t", drives[i].f3);
      printf("%s\t\t\naa\t", drives[i].f4);
      printf("%s\t\t\naa\t", drives[i].f5);
    }
    return 0;
  }
`
	if err := ioutil.WriteFile(d+"/main.c", []byte(content), 0o755); err != nil {
		return nil, err
	}

	compile := exec.Command("cc", d+"/main.c", "-o", d+"/main")
	compile.Stdout = os.Stdout
	compile.Stderr = os.Stderr
	if err := compile.Run(); err != nil {
		return nil, err
	}

	return exec.Command(d + "/main").Output()
}

func printDeviceDatabase(db []byte) error {
	lines := strings.Split(string(db), "\t\t\naa\t")

	if !strings.HasPrefix(lines[0], "VERSION:") {
		return fmt.Errorf("'VERSION:' must be the first line in the database")
	}
	if !strings.HasPrefix(lines[5], "DEFAULT") {
		return fmt.Errorf("'DEFAULT' must be the first line in the database")
	}

	f, err := os.Create(*outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	defaultNames := make(map[int]string)

	f.WriteString(`package smart

    var ataDefaultAttributes = `)
	if err := printPresets(f, lines[9], false, false, defaultNames); err != nil {
		return err
	}

	f.WriteString(`

    var ataDevicesDatabase = []ataDeviceInfo{
`)

	i := 10
	for i < len(lines)-1 {
		dev := lines[i : i+5]
		i += 5
		name := dev[0]
		if strings.HasPrefix(name, "USB: ") {
			continue // skip USB device
		}

		if err := printDevice(f, dev, defaultNames); err != nil {
			return err
		}
		f.WriteString(",\n")
	}

	f.WriteString(`}`)

	return nil
}

func printDevice(f *os.File, dev []string, defaultNames map[int]string) error {
	modelFamily := dev[0]
	modelRegexp := dev[1]
	firmwareRegexp := dev[2]
	// warningmMessage := dev[3]
	presets := dev[4]

	f.WriteString("{`")
	f.WriteString(modelFamily)
	f.WriteString("`,`")
	f.WriteString(modelRegexp)
	f.WriteString("`,`")
	f.WriteString(firmwareRegexp)
	f.WriteString("`, ")
	if err := printPresets(f, presets, true, true, defaultNames); err != nil {
		return err
	}
	f.WriteString("}")

	return nil
}

func printPresets(f *os.File, presets string, printFirmwareBugs bool, useAttrNames bool, names map[int]string) error {
	presets = strings.TrimSpace(presets)
	if presets == "" {
		f.WriteString("nil,0")
		return nil
	}

	f.WriteString("map[int]ataDeviceAttr{")

	parts := strings.Split(presets, " ")
	i := 0

	firmwareBug := []string{}

	for i < len(parts) {
		key := parts[i]
		value := parts[i+1]
		i += 2

		if newValue, ok := oldAttrValue[value]; ok {
			value = newValue
		}

		switch key {
		case "-v":
			restriction := ""

			att := strings.Split(value, ",")
			if len(att) == 4 {
				if att[3] == "HDD" {
					restriction = "ataDeviceAttributeRestrictionHDDOnly"
					att = att[:3]
				} else if att[3] == "SSD" {
					restriction = "ataDeviceAttributeRestrictionSSDOnly"
					att = att[:3]
				}
			}
			if restriction == "" {
				restriction = "0"
			}

			if len(att) < 2 || len(att) > 3 {
				return fmt.Errorf("invalid number of attribute fields for '%s'", value)
			}

			rawType := att[1]
			byteOrder := ""
			if idx := strings.Index(rawType, ":"); idx != -1 {
				byteOrder = rawType[idx+1:]
				rawType = rawType[:idx]
			}

			if rawType[len(rawType)-1] == '+' {
				// TODO: understand what does this "ATTRFLAG_INCREASING" modifier really mean
				// for now we just ignore it
				rawType = rawType[:len(rawType)-1]
			}

			typ, ok := attributeTypeId[rawType]
			if !ok {
				return fmt.Errorf("unknown attribute type: %s", rawType)
			}

			name := ""
			if len(att) > 2 {
				name = att[2]
			}

			id, err := strconv.Atoi(att[0])
			if err != nil {
				return fmt.Errorf("invalid attribute id for '%s'", value)
			}

			if useAttrNames {
				if name == "" {
					name = names[id]
				}
			} else {
				names[id] = name
			}

			f.WriteString(strconv.Itoa(id))
			f.WriteString(":{`")
			f.WriteString(name)
			f.WriteString("`,")
			f.WriteString(typ)
			f.WriteString(",`")
			f.WriteString(byteOrder)
			f.WriteString("`,")
			f.WriteString(restriction)
			f.WriteString("},")
		case "-F":
			if b, ok := firmwareBugById[value]; ok {
				firmwareBug = append(firmwareBug, b)
			} else {
				return fmt.Errorf("unknown firmware bug id: %s", value)
			}
		}
	}
	f.WriteString("}")

	if printFirmwareBugs {
		f.WriteString(",")
		if len(firmwareBug) == 0 {
			firmwareBug = []string{"0"}
		}
		f.WriteString(strings.Join(firmwareBug, "|"))
	}

	return nil
}

var firmwareBugById = map[string]string{
	"nologdir":  "ataFirmwareBugNoLogDir",
	"samsung":   "ataFirmwareBugSamsung",
	"samsung2":  "ataFirmwareBugSamsung2",
	"samsung3":  "ataFirmwareBugSamsung3",
	"xerrorlba": "ataFirmwareBugXErrorLBA",
}

var attributeTypeId = map[string]string{
	"raw8":         "AtaDeviceAttributeTypeRaw8",
	"raw16":        "AtaDeviceAttributeTypeRaw16",
	"raw48":        "AtaDeviceAttributeTypeRaw48",
	"hex48":        "AtaDeviceAttributeTypeHex48",
	"raw56":        "AtaDeviceAttributeTypeRaw56",
	"hex56":        "AtaDeviceAttributeTypeHex56",
	"raw64":        "AtaDeviceAttributeTypeRaw64",
	"hex64":        "AtaDeviceAttributeTypeHex64",
	"raw16(raw16)": "AtaDeviceAttributeTypeRaw16OptRaw16",
	"raw16(avg16)": "AtaDeviceAttributeTypeRaw16OptAvg16",
	"raw24(raw8)":  "AtaDeviceAttributeTypeRaw24OptRaw8",
	"raw24/raw24":  "AtaDeviceAttributeTypeRaw24DivRaw24",
	"raw24/raw32":  "AtaDeviceAttributeTypeRaw24DivRaw32",
	"sec2hour":     "AtaDeviceAttributeTypeSec2Hour",
	"min2hour":     "AtaDeviceAttributeTypeMin2Hour",
	"halfmin2hour": "AtaDeviceAttributeTypeHalfMin2Hour",
	"msec24hour32": "AtaDeviceAttributeTypeMsec24Hour32",
	"tempminmax":   "AtaDeviceAttributeTypeTempMinMax",
	"temp10x":      "AtaDeviceAttributeTypeTemp10X",
}

// upstream database contains legacy attribute names as well
var oldAttrValue = map[string]string{
	"9,halfminutes":               "9,halfmin2hour,Power_On_Half_Minutes",
	"9,minutes":                   "9,min2hour,Power_On_Minutes",
	"9,seconds":                   "9,sec2hour,Power_On_Seconds",
	"9,temp":                      "9,tempminmax,Temperature_Celsius",
	"192,emergencyretractcyclect": "192,raw48,Emerg_Retract_Cycle_Ct",
	"193,loadunload":              "193,raw24/raw24",
	"194,10xCelsius":              "194,temp10x,Temperature_Celsius_x10",
	"194,unknown":                 "194,raw48,Unknown_Attribute",
	"197,increasing":              "197,raw48+,Total_Pending_Sectors",
	"198,offlinescanuncsectorct":  "198,raw48,Offline_Scan_UNC_SectCt",
	"198,increasing":              "198,raw48+,Total_Offl_Uncorrectabl",
	"200,writeerrorcount":         "200,raw48,Write_Error_Count",
	"201,detectedtacount":         "201,raw48,Detected_TA_Count",
	"220,temp":                    "220,tempminmax,Temperature_Celsius",
}
