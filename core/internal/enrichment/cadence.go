package enrichment

import (
	"fmt"
	"time"
)

// defaultDrainCadence is how often the worker wakes to check the queue when
// the operator configured nothing (TECHNICAL-DECISIONS.md §1.29). An empty
// queue makes a tick nearly free, so this trades only freshness against
// pointless wakeups.
const defaultDrainCadence = 15 * time.Minute

// DrainCadence resolves the worker's tick interval: an explicit config value
// wins over the default; anything else is a broken config reported loudly at
// startup rather than silently ignored.
func DrainCadence(configured string) (time.Duration, error) {
	if configured == "" {
		return defaultDrainCadence, nil
	}
	d, err := time.ParseDuration(configured)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("enrichment: drain-cadence %q is not a positive duration", configured)
	}
	return d, nil
}
