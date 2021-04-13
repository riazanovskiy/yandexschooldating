package main_test

import (
	"testing"
	"time"

	main "yandexschooldating"

	"yandexschooldating/clock"

	"github.com/stretchr/testify/require"
)

func TestChooseNextMatchTimerDate(t *testing.T) {
	clock := clock.Fake{Current: time.Date(2021, 1, 5, 4, 20, 0, 0, time.UTC)}

	next := main.ChooseNextMatchTimerDate(&clock)
	require.Equal(t, time.Date(2021, 1, 11, 0, 0, 0, 0, time.UTC), next)

	clock.Current = next
	next = main.ChooseNextMatchTimerDate(&clock)
	require.Equal(t, time.Date(2021, 1, 18, 0, 0, 0, 0, time.UTC), next)

	clock.Current = next
	next = main.ChooseNextMatchTimerDate(&clock)
	require.Equal(t, time.Date(2021, 1, 25, 0, 0, 0, 0, time.UTC), next)

	clock.Current = next
	next = main.ChooseNextMatchTimerDate(&clock)
	require.Equal(t, time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC), next)

	clock.Current = next
	next = main.ChooseNextMatchTimerDate(&clock)
	require.Equal(t, time.Date(2021, 2, 8, 0, 0, 0, 0, time.UTC), next)
}

func TestInitMatchTimerChan(t *testing.T) {
	clock := clock.Fake{Current: time.Date(2021, 1, 31, 23, 59, 56, 0, time.UTC)}
	result := main.InitMatchTimerChan(&clock)

	start := time.Now()
	<-result
	elapsed := time.Since(start).Seconds()
	require.LessOrEqual(t, 3.0, elapsed)
	require.LessOrEqual(t, elapsed, 5.0)
}
