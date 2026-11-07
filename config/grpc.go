package config

type GRPC struct {
	Port string `env:"GRPC_PORT" envDefault:"8081"`
}
