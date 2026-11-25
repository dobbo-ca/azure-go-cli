package support

import (
	_ "embed"
	"encoding/xml"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// CLDR release-49 data (see cldr/LICENSE).
//
//go:embed cldr/windowsZones.xml
var windowsZonesXML []byte

//go:embed cldr/timezone.xml
var timezoneXML []byte

type zoneTables struct {
	byIANA map[string]string
	names  []string
}

var zones = sync.OnceValue(func() zoneTables {
	var wz struct {
		Maps []struct {
			Other string `xml:"other,attr"`
			Type  string `xml:"type,attr"`
		} `xml:"windowsZones>mapTimezones>mapZone"`
	}
	var tz struct {
		Keys []struct {
			Name  string `xml:"name,attr"`
			Types []struct {
				Alias string `xml:"alias,attr"`
			} `xml:"type"`
		} `xml:"keyword>key"`
	}
	// The embedded files are fixed, so a parse failure is a build-time bug.
	if err := xml.Unmarshal(windowsZonesXML, &wz); err != nil {
		panic(err)
	}
	if err := xml.Unmarshal(timezoneXML, &tz); err != nil {
		panic(err)
	}
	t := zoneTables{byIANA: map[string]string{}}
	seen := map[string]bool{}
	for _, m := range wz.Maps {
		if !seen[m.Other] {
			seen[m.Other] = true
			t.names = append(t.names, m.Other)
		}
		for _, id := range strings.Fields(m.Type) {
			if _, ok := t.byIANA[id]; !ok {
				t.byIANA[id] = m.Other
			}
		}
	}
	sort.Strings(t.names)
	for _, k := range tz.Keys {
		if k.Name != "tz" {
			continue
		}
		for _, ty := range k.Types {
			group := strings.Fields(ty.Alias)
			win := ""
			for _, id := range group {
				if w, ok := t.byIANA[id]; ok {
					win = w
					break
				}
			}
			if win == "" {
				continue
			}
			for _, id := range group {
				if _, ok := t.byIANA[id]; !ok {
					t.byIANA[id] = win
				}
			}
		}
	}
	return t
})

// windowsZone maps an IANA zone (or any of its CLDR aliases) to a Windows zone name.
func windowsZone(iana string) (string, bool) {
	w, ok := zones().byIANA[iana]
	return w, ok
}

// windowsZoneNames lists the distinct Windows zone names, sorted.
func windowsZoneNames() []string { return zones().names }

// ianaFromTZ finds the local IANA zone name from $TZ or the /etc/localtime
// link target. It returns "" when unknown, which makes the UI show the picker.
func ianaFromTZ(tzEnv, localtimeLink string) string {
	if runtime.GOOS == "windows" {
		return ""
	}
	v := strings.TrimPrefix(tzEnv, ":")
	if v == "Local" {
		return ""
	}
	if v == "" {
		v = localtimeLink
	}
	if i := strings.LastIndex(v, "zoneinfo/"); i >= 0 {
		return v[i+len("zoneinfo/"):]
	}
	return v
}

func localIANAZone() string {
	link, _ := os.Readlink("/etc/localtime")
	return ianaFromTZ(os.Getenv("TZ"), link)
}
