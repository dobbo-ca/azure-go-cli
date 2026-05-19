/*
Copyright © 2024 netr0m <netr0m@pm.me>
*/
package pimvendor

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseDateTime(t *testing.T) {
	now := time.Now().Local()
	currentDate := now.Format("2006-01-02")
	currentTZ := now.Format("-07:00")
	// dec31TZ is the local TZ offset on Dec 31 2024 specifically — different from
	// currentTZ when "now" is in DST but Dec 31 is in standard time (or vice
	// versa). Pinning the expected value to the parsed date's actual offset
	// avoids flakiness across DST transitions.
	dec31TZ := time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local).Format("-07:00")
	errMsg := "resulting startDateTime does not match expected value"

	dateOnly, _ := parseDateTime("31/12/2024", "")
	timeOnly, _ := parseDateTime("", "13:37")
	dateTime, _ := parseDateTime("31/12/2024", "13:37")

	assert.Equal(t, fmt.Sprintf("2024-12-31T00:00:00%s", dec31TZ), dateOnly, errMsg)
	assert.Equal(t, fmt.Sprintf("%sT13:37:00%s", currentDate, currentTZ), timeOnly, errMsg)
	assert.Equal(t, fmt.Sprintf("2024-12-31T13:37:00%s", dec31TZ), dateTime, errMsg)
}
