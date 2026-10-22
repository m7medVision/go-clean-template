package config

// GRPC -.
type grpc struct {
	Port string `env:"GRPC_PORT" envDefault:"8081"`
}
