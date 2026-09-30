package health

import (
	"fmt"
	"strings"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

// Status is the health of one check or of the whole service.
type Status string

// Health statuses, from best to worst.
const (
	StatusUp       Status = "up"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
)

// CheckInventory is the name of the stock check.
const CheckInventory = "inventory"

// Check is the result of one health check.
type Check struct {
	Name       string // dependency name or CheckInventory
	Status     Status
	Reason     string                     // empty when Status is up
	Dependency fulfillment.DependencyName // the dependency this check reads; empty for the inventory check
	OutOfStock []fulfillment.SKU          // inventory check: products with zero stock, in SKU order; nil otherwise
}

// Report is the readiness of the service.
type Report struct {
	Status Status  // worst status of all checks
	Checks []Check // dependencies in snapshot order, then inventory
}

// Evaluate derives the readiness report from a snapshot. It is a pure function.
func Evaluate(s fulfillment.Snapshot) Report {
	checks := make([]Check, 0, len(s.Dependencies)+1)
	for _, d := range s.Dependencies {
		checks = append(checks, dependencyCheck(d))
	}
	checks = append(checks, inventoryCheck(s.Products))

	overall := StatusUp
	for _, c := range checks {
		overall = worse(overall, c.Status)
	}
	return Report{Status: overall, Checks: checks}
}

func dependencyCheck(d fulfillment.Dependency) Check {
	name := string(d.Name)
	switch d.Mode {
	case fulfillment.ModeSlow:
		return Check{
			Name: name, Status: StatusDegraded, Dependency: d.Name,
			Reason: fmt.Sprintf("%s is slow: calls take %d ticks", name, fulfillment.SlowLatencyTicks),
		}
	case fulfillment.ModeOutage:
		return Check{
			Name: name, Status: StatusDown, Dependency: d.Name,
			Reason: fmt.Sprintf("%s is in outage: calls fail", name),
		}
	default:
		return Check{Name: name, Status: StatusUp, Dependency: d.Name}
	}
}

func inventoryCheck(products []fulfillment.Product) Check {
	var empty []fulfillment.SKU
	var names []string
	for _, p := range products {
		if p.Stock == 0 {
			empty = append(empty, p.SKU)
			names = append(names, string(p.SKU))
		}
	}
	if len(empty) == 0 {
		return Check{Name: CheckInventory, Status: StatusUp}
	}
	return Check{
		Name: CheckInventory, Status: StatusDegraded, OutOfStock: empty,
		Reason: "out of stock: " + strings.Join(names, ", "),
	}
}

func worse(a, b Status) Status {
	if rank(b) > rank(a) {
		return b
	}
	return a
}

func rank(s Status) int {
	switch s {
	case StatusDown:
		return 2
	case StatusDegraded:
		return 1
	default:
		return 0
	}
}
