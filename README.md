# Inspector

Inspector is a Go toolkit that helps you understand a running system from the outside. It reads what the system exposes about itself (status endpoints, APIs, metrics), turns that into a model of connected entities, and shows the model as web pages you can explore.

It answers three questions:

- **What can the system tell us about itself?** Collect signals from its HTTP endpoints.
- **How is this connected?** Follow the links between orders, products, dependencies and health checks.
- **How does this system actually work?** See what is degraded or down, and why.

## Try it

Run the workbench. It starts a simulated order fulfillment service and inspects it:

```sh
task workbench
```

Open http://localhost:8080. The page has three tabs:

- **Dashboard**: the health of every kind of entity, what needs attention and why.
- **Model**: every entity, grouped by kind. Open one to see its state, properties and links, and follow the links.
- **Raw**: the signals as they were collected.

The buttons in the header put the simulated payment gateway or warehouse into `slow` or `outage`. Watch the effect spread to the health checks and orders. The pages refresh every 2 seconds. See [harness/README.md](harness/README.md) for more.

## Use it as a library

```sh
go get github.com/miroslav-matejovsky/inspector
```

A Source collects signals, a Model is built from them, and a View renders the Model:

```go
src, err := source.Open(ctx, source.Config{
	Targets:      []source.Target{{Name: "orders", URL: "http://localhost:9000/api/orders"}},
	Interval:     5 * time.Second,
	Timeout:      2 * time.Second,
	Retention:    10 * time.Minute,
	MaxBodyBytes: 1 << 20,
	DatabasePath: "data/source.db",
}, &http.Client{}, slog.Default())
if err != nil {
	return err
}
defer src.Close()
// src.Run(ctx) collects on every Interval; src.Collect(ctx) collects once.

summaries, err := src.Summary(ctx)
if err != nil {
	return err
}
m := model.Build(summaries, func(t source.Target) model.Interpreter {
	if t.Name == "orders" {
		return interpretOrders // your func(source.Signal) ([]model.Entity, error)
	}
	return nil
})
html, err := dashboard.Render(m, "/model")
```

You write an interpreter for each kind of signal. It turns a response body into entities with state and links. The views work with any model. The workbench interpreters in [harness/workbench/inspectedmodel.go](harness/workbench/inspectedmodel.go) are a complete example.

## Documentation

- [docs/](docs/README.md): architecture and development.
- [dev/principles.md](dev/principles.md): the principles behind the design.
- `go doc` on any package, for example `go doc ./model`.
