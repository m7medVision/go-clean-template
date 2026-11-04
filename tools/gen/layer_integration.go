package main

// planIntegration plans the integration test every domain gets, integration-test/<name>_test.go. It
// signs a user in, then checks every operation on each transport the domain has against the
// running app and worker: until the handlers are written, each answers "not implemented".
func planIntegration(pl *planner) {
	pl.create("integration-test/*.tmpl")
}
