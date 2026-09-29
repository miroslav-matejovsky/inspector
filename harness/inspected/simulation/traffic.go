package simulation

import (
	"math/rand/v2"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

// maxGeneratedQuantity is the largest quantity of a generated order.
const maxGeneratedQuantity = 5

// traffic generates order requests from a seeded random source. It is owned by
// the Run goroutine and is deterministic for a given seed.
type traffic struct {
	rng           *rand.Rand
	ordersPerTick int
	skus          []fulfillment.SKU // catalog SKUs in sorted order
}

func newTraffic(seed uint64, ordersPerTick int, skus []fulfillment.SKU) *traffic {
	return &traffic{rng: rand.New(rand.NewPCG(seed, seed)), ordersPerTick: ordersPerTick, skus: skus}
}

// next returns the next generated order request.
func (t *traffic) next() fulfillment.OrderRequest {
	return fulfillment.OrderRequest{
		SKU:      t.skus[t.rng.IntN(len(t.skus))],
		Quantity: 1 + t.rng.IntN(maxGeneratedQuantity),
	}
}
