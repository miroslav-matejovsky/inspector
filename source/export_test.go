package source

import "time"

// SetClock replaces the clock of s. Only tests use it.
func SetClock(s *Source, now func() time.Time) { s.now = now }
