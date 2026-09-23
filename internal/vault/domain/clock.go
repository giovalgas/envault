package domain

import "time"

type Clock func() time.Time

func (c Clock) Now() time.Time {
	now := time.Now
	if c != nil {
		now = c
	}
	return now().UTC().Truncate(time.Second)
}
