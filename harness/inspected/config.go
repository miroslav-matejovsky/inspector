package inspected

import (
	"fmt"
	"time"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/simulation"
)

// Config configures the inspected service. Every field is required and has no
// default.
type Config struct {
	Seed           uint64        // seed of the traffic generator
	OrdersPerTick  int           // simulated orders placed per tick, 0..simulation.MaxOrdersPerTick
	TickInterval   time.Duration // wall-clock time between clock ticks, must be positive
	RequestTimeout time.Duration // max wait for a simulation reply per HTTP request, must be positive
}

// Validate reports whether the config is usable.
func (c Config) Validate() error {
	if c.TickInterval <= 0 {
		return fmt.Errorf("inspected: tick interval must be positive, got %s", c.TickInterval)
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("inspected: request timeout must be positive, got %s", c.RequestTimeout)
	}
	if err := (simulation.Config{Seed: c.Seed, OrdersPerTick: c.OrdersPerTick}).Validate(); err != nil {
		return fmt.Errorf("inspected: %w", err)
	}
	return nil
}
