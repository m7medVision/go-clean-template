package flags

import (
	"github.com/m7medvision/go-clean-template/internal/usecase/flags/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/flags"
)

type Deps struct {
	Flags *flags.Client
}

func Register(qs *cqrs.Queries, d Deps) {
	cqrs.HandleQuery(qs, query.GetFlagsHandler{Flags: d.Flags})
}
