package fulfillment

import "fmt"

// DependencyName names a downstream system the service calls.
type DependencyName string

// Downstream dependencies.
const (
	DependencyPaymentGateway DependencyName = "payment-gateway" // called while an order is pending
	DependencyWarehouse      DependencyName = "warehouse"       // called while an order is paid
)

// DependencyNames returns every dependency in name order. Every call returns
// a new slice.
func DependencyNames() []DependencyName {
	return []DependencyName{DependencyPaymentGateway, DependencyWarehouse}
}

// DependencyMode is the simulated behavior of a dependency.
type DependencyMode string

// Dependency modes.
const (
	ModeHealthy DependencyMode = "healthy" // calls succeed after HealthyLatencyTicks
	ModeSlow    DependencyMode = "slow"    // calls succeed after SlowLatencyTicks
	ModeOutage  DependencyMode = "outage"  // calls fail
)

// DependencyModes returns every mode. Every call returns a new slice.
func DependencyModes() []DependencyMode {
	return []DependencyMode{ModeHealthy, ModeSlow, ModeOutage}
}

// ParseDependencyMode converts s to a DependencyMode. An unknown value returns
// an error wrapping ErrInvalidDependencyMode.
func ParseDependencyMode(s string) (DependencyMode, error) {
	switch mode := DependencyMode(s); mode {
	case ModeHealthy, ModeSlow, ModeOutage:
		return mode, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidDependencyMode, s)
}

// Dependency is a downstream system and its current mode.
type Dependency struct {
	Name DependencyName
	Mode DependencyMode
}
