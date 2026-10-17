// Package viewings holds the rules for booking a viewing at the sales suite:
// which days it opens, its slot times and how many parties fit in a slot.
package viewings

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Nairobi is the sales suite's time zone.
var Nairobi = func() *time.Location {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return loc
}()

// Length is how long a viewing lasts.
const Length = 45 * time.Minute

// BookAhead is how far ahead a visitor may book.
const BookAhead = 14

// MinNotice is the least notice a booking needs.
const MinNotice = 2 * time.Hour

// Config is the suite's opening pattern, which a manager can change.
type Config struct {
	Days     []time.Weekday
	Times    []string
	Capacity int
}

// Default is how the suite opens: Tuesday to Sunday, nine slots a day, two
// parties in each (there are two show apartments to walk round).
func Default() Config {
	return Config{
		Days:     []time.Weekday{time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday},
		Times:    []string{"10:00", "10:45", "11:30", "12:15", "13:00", "14:00", "14:45", "15:30", "16:15"},
		Capacity: 2,
	}
}

// Opens reports whether the suite opens on the day of t.
func (c Config) Opens(t time.Time) bool {
	wd := t.In(Nairobi).Weekday()
	for _, d := range c.Days {
		if d == wd {
			return true
		}
	}
	return false
}

// SlotsOn lists the slot start times on the day of t, if the suite opens.
func (c Config) SlotsOn(t time.Time) []time.Time {
	day := t.In(Nairobi)
	if !c.Opens(day) {
		return nil
	}
	var out []time.Time
	for _, hm := range c.Times {
		h, m, err := ParseClock(hm)
		if err != nil {
			continue
		}
		out = append(out, time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, Nairobi))
	}
	return out
}

// IsSlot reports whether t is the start of one of the suite's slots.
func (c Config) IsSlot(t time.Time) bool {
	for _, s := range c.SlotsOn(t) {
		if s.Equal(t) {
			return true
		}
	}
	return false
}

// ParseClock reads "14:45".
func ParseClock(s string) (int, int, error) {
	h, m, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, fmt.Errorf("viewings: %q is not HH:MM", s)
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, fmt.Errorf("viewings: %q is not HH:MM", s)
	}
	return hh, mm, nil
}

// Validate reports a pattern a manager should not be able to save.
func (c Config) Validate() error {
	if len(c.Days) == 0 {
		return fmt.Errorf("choose at least one day")
	}
	if len(c.Times) == 0 {
		return fmt.Errorf("choose at least one time")
	}
	if c.Capacity < 1 || c.Capacity > 6 {
		return fmt.Errorf("a slot takes between 1 and 6 parties")
	}
	seen := map[string]bool{}
	for _, t := range c.Times {
		h, _, err := ParseClock(t)
		if err != nil {
			return err
		}
		if h < 7 || h > 19 {
			return fmt.Errorf("%s is outside 07:00 to 19:00", t)
		}
		if seen[t] {
			return fmt.Errorf("%s is listed twice", t)
		}
		seen[t] = true
	}
	return nil
}

// Encode writes the pattern for the settings table.
func (c Config) Encode() (days, times, capacity string) {
	ds := make([]string, len(c.Days))
	for i, d := range c.Days {
		ds[i] = strconv.Itoa(int(d))
	}
	ts := append([]string(nil), c.Times...)
	sort.Strings(ts)
	return strings.Join(ds, ","), strings.Join(ts, ","), strconv.Itoa(c.Capacity)
}

// Decode reads the pattern from the settings table, falling back to Default.
func Decode(days, times, capacity string) Config {
	c := Default()
	if days != "" {
		var ds []time.Weekday
		for _, s := range strings.Split(days, ",") {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 6 {
				ds = append(ds, time.Weekday(n))
			}
		}
		if len(ds) > 0 {
			c.Days = ds
		}
	}
	if times != "" {
		var ts []string
		for _, s := range strings.Split(times, ",") {
			if _, _, err := ParseClock(s); err == nil {
				ts = append(ts, s)
			}
		}
		if len(ts) > 0 {
			c.Times = ts
		}
	}
	if n, err := strconv.Atoi(capacity); err == nil && n > 0 {
		c.Capacity = n
	}
	return c
}

// SlotKey is a slot's identity, for counting bookings in it.
func SlotKey(t time.Time) string {
	return t.In(Nairobi).Format("2006-01-02 15:04")
}
