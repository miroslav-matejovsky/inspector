# Harness

Development tooling for running Inspector against something to inspect.

## workbench

`workbench/` is a Go package that serves a web page with two panels: **inspected** and **inspector**. Both are empty for now. Start it with `task workbench` and open http://localhost:8080.

The entry point is `cmd/workbench`. The listen address is passed with `-addr` and has no default.

## inspected

`inspected/` is reserved for the system under inspection. It is empty for now.
