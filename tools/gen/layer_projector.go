package main

// planProjector plans the projector every domain gets, which writes the read model's table from
// the domain's events, and its test against a fake of sqlc's Querier. The worker runs it once it
// is in subscribers, which the use-case layer wires.
func planProjector(pl *planner) {
	pl.create("internal/repo/persistent/{{.Name.Pkg}}/projector*.tmpl")
}
