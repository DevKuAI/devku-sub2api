package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDesktopConversationDurationMS(t *testing.T) {
	for _, tc := range []struct {
		start, stop time.Time
		want        int64
	}{
		{time.Date(2026, 9, 29, 0, 0, 0, 900_000_000, time.UTC), time.Date(2026, 9, 29, 0, 0, 1, 500_000, time.UTC), 100},
		{time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC), 0},
		{time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), 315537897599000},
	} {
		require.Equal(t, tc.want, desktopConversationDurationMS(tc.start, tc.stop))
	}
}
