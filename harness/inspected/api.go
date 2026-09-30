package inspected

import (
	"fmt"
	"net/http"

	"github.com/miroslav-matejovsky/inspector/harness/inspected/fulfillment"
)

func (a *App) handleListProducts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.requestContext(r)
	defer cancel()

	snap, err := a.sim.Snapshot(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	products := make([]productResource, 0, len(snap.Products))
	for _, p := range snap.Products {
		products = append(products, productResourceOf(p))
	}
	writeJSON(w, http.StatusOK, productListResource{Products: products})
}

func (a *App) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.requestContext(r)
	defer cancel()

	snap, err := a.sim.Snapshot(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	sku := r.PathValue("sku")
	p, ok := snap.Product(fulfillment.SKU(sku))
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "product_not_found", fmt.Sprintf("product %q not found", sku))
		return
	}
	writeJSON(w, http.StatusOK, productResourceOf(p))
}

// handleListOrders lists retained orders in placement order. The optional
// query parameters status and sku filter the list.
func (a *App) handleListOrders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var status fulfillment.OrderStatus
	if s := query.Get("status"); s != "" {
		parsed, err := fulfillment.ParseOrderStatus(s)
		if err != nil {
			writeError(w, err)
			return
		}
		status = parsed
	}
	sku := fulfillment.SKU(query.Get("sku"))

	ctx, cancel := a.requestContext(r)
	defer cancel()
	snap, err := a.sim.Snapshot(ctx)
	if err != nil {
		writeError(w, err)
		return
	}

	orders := make([]orderResource, 0, len(snap.Orders))
	for _, o := range snap.Orders {
		if (status == "" || o.Status == status) && (sku == "" || o.SKU == sku) {
			orders = append(orders, orderResourceOf(o))
		}
	}
	writeJSON(w, http.StatusOK, orderListResource{Orders: orders})
}

func (a *App) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.requestContext(r)
	defer cancel()

	snap, err := a.sim.Snapshot(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	id := r.PathValue("id")
	o, ok := snap.Order(fulfillment.OrderID(id))
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "order_not_found", fmt.Sprintf("order %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, orderResourceOf(o))
}

func (a *App) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	var req placeOrderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}

	ctx, cancel := a.requestContext(r)
	defer cancel()
	o, err := a.sim.PlaceOrder(ctx, fulfillment.OrderRequest{SKU: fulfillment.SKU(req.SKU), Quantity: req.Quantity})
	if err != nil {
		writeError(w, err)
		return
	}
	res := orderResourceOf(o)
	w.Header().Set("Location", res.Links.Self)
	writeJSON(w, http.StatusCreated, res)
}

func (a *App) handleListDependencies(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.requestContext(r)
	defer cancel()

	snap, err := a.sim.Snapshot(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	deps := make([]dependencyResource, 0, len(snap.Dependencies))
	for _, d := range snap.Dependencies {
		deps = append(deps, dependencyResourceOf(d))
	}
	writeJSON(w, http.StatusOK, dependencyListResource{Dependencies: deps})
}

func (a *App) handleGetDependency(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.requestContext(r)
	defer cancel()

	snap, err := a.sim.Snapshot(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	name := r.PathValue("name")
	for _, d := range snap.Dependencies {
		if string(d.Name) == name {
			writeJSON(w, http.StatusOK, dependencyResourceOf(d))
			return
		}
	}
	writeErrorCode(w, http.StatusNotFound, "dependency_not_found", fmt.Sprintf("dependency %q not found", name))
}
