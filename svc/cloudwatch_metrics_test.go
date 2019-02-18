package svc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_splitTime(t *testing.T) {
	tests := []struct {
		Start, End time.Time
		Period     time.Duration
		Expected   [][2]time.Time
	}{
		{
			Start: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC), Period: time.Minute,
			Expected: [][2]time.Time{
				[2]time.Time{time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)},
			},
		},
		{
			Start: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2000, 1, 3, 0, 0, 0, 0, time.UTC), Period: time.Minute,
			Expected: [][2]time.Time{
				[2]time.Time{time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)},
				[2]time.Time{time.Date(2000, 1, 2, 0, 1, 0, 0, time.UTC), time.Date(2000, 1, 3, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	for _, test := range tests {
		result := splitTime(test.Start, test.End, test.Period)
		assert.Equal(t, len(test.Expected), len(result))
		for i, v := range result {
			ext := test.Expected[i]
			assert.Equal(t, true, ext[0].Equal(v[0]))
			assert.Equal(t, true, ext[1].Equal(v[1]))
		}
	}
}
