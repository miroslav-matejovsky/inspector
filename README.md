# Inspector

Inspector is a Go library for inspecting running systems. Its principles are in [dev/principles.md](dev/principles.md).

## Harness

`harness/` holds the development workbench: a web page with a single workbench panel, plus a simulated order fulfillment service that exposes a JSON API, health checks and Prometheus metrics for Inspector to work against. Start it with `task workbench`. See [harness/README.md](harness/README.md).
