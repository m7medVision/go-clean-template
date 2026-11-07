package rules

import "github.com/m7medvision/go-clean-template/tools/gen/names"

// The app's layout, as far as the generator looks it up: where each transport keeps the domains'
// controllers, where the API's descriptions are, and the one gRPC service named otherwise.
// Everything else it writes is laid out under templates/ as the repository is.

// ProtoDir holds the .proto files, which are one proto package, grpc.v1.
const ProtoDir = "docs/proto/v1"

// _openAPI is the OpenAPI document make openapi exports from the REST API: every REST operation
// the app registers.
const _openAPI = "docs/openapi/v1.json"

// transport is one transport a domain can have: its name, as --transports takes it, and root, the
// folder that holds its controllers.
type transport struct {
	name, root string
}

func transports() []transport {
	return []transport{
		{name: "rest", root: "internal/controller/restapi/v1"},
		{name: "grpc", root: "internal/controller/grpc/v1"},
		{name: "graphql", root: "internal/controller/graphql"},
	}
}

// Transports are the transports a domain can have, in the order the generator lists them.
func Transports() []string {
	var all []string

	for _, t := range transports() {
		all = append(all, t.name)
	}

	return all
}

// ControllerRoot is the folder that holds the controllers of transport t: one folder per domain,
// beside the packages the domains share and, for GraphQL, resolver.go, which lists them. It is
// empty for a transport Transports does not list.
func ControllerRoot(t string) string {
	for _, tr := range transports() {
		if tr.name == t {
			return tr.root
		}
	}

	return ""
}

// ControllerDir is the folder of the domain pkg's controllers on transport t. Given "{{.Name.Pkg}}"
// for pkg, it is that folder as the templates name it.
func ControllerDir(t, pkg string) string {
	return ControllerRoot(t) + "/" + pkg
}

// SchemaDir holds the GraphQL schema files, which gqlgen reads as one schema.
func SchemaDir() string {
	return ControllerRoot("graphql") + "/schema"
}

// GRPCService returns the gRPC service the domain n's controller serves and the .proto file that
// declares it: <Name>Service in docs/proto/v1/<name>.proto. User's is the one exception, kept on
// purpose: it serves AuthService, from auth.proto, a name its clients already call.
func GRPCService(n *names.Name) (service, file string) {
	if n.Kebab == "user" {
		return "AuthService", ProtoDir + "/auth.proto"
	}

	return n.Go + "Service", ProtoDir + "/" + n.Snake + ".proto"
}
