package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMustLoadLocation(t *testing.T) {
	assert.Panics(t, func() {
		MustLoadLocation("Invalid Location")
	})

	assert.NotPanics(t, func() {
		MustLoadLocation("UTC")
	})
}
func TestGetDayInZone(t *testing.T) {
	WithClockTime(t, mustParseTime("2015-05-01T19:00:00Z"), func(t *testing.T) {
		assert.Equal(t, "Friday", GetDayInZone(MustLoadLocation("UTC")))
		assert.Equal(t, "Friday", GetDayInZone(MustLoadLocation("America/Los_Angeles")))
		assert.Equal(t, "Saturday", GetDayInZone(MustLoadLocation("Asia/Tokyo")))
	})
}

func makeMockClock(t *time.Time) ClockFunc {
	return func() *time.Time {
		return t
	}
}

func mustParseTime(str string) *time.Time {
	t, err := time.Parse(time.RFC3339, str)

	if err != nil {
		panic(err)
	}

	return &t
}

func WithClockTime(t *testing.T, ct *time.Time, cb func(*testing.T)) {
	orig := Clock
	Clock = makeMockClock(ct)

	defer func() {
		Clock = orig
	}()

	cb(t)
}
