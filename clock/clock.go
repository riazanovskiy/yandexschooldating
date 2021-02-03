package clock

import "time"

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func NewRealClock() Clock {
	return realClock{}
}

func (realClock) Now() time.Time { return time.Now() }

type Fake struct {
	Current time.Time
}

func (f *Fake) Now() time.Time {
	return f.Current
}
