package main

import (
	"testing"
	"time"
)

func TestRetryDelayIsBounded(t *testing.T) {
	cases := []struct {
		failure int
		want    time.Duration
	}{
		{failure: 0, want: time.Second},
		{failure: 1, want: time.Second},
		{failure: 2, want: 2 * time.Second},
		{failure: 3, want: 4 * time.Second},
		{failure: 4, want: 8 * time.Second},
		{failure: 5, want: 8 * time.Second},
		{failure: 40, want: 8 * time.Second},
	}
	for _, test := range cases {
		if got := retryDelay(test.failure); got != test.want {
			t.Errorf("retryDelay(%d) = %s, se esperaba %s", test.failure, got, test.want)
		}
	}
}
