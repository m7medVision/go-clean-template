package task_test

import (
	"context"
	"net/http"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
)

type fakeTask struct {
	usecase.Task

	stored entity.Task
	err    error

	listed    []entity.Task
	gotCaller entity.Principal
	gotStatus *entity.TaskStatus
	gotLimit  int
	gotOffset int
}

func (f *fakeTask) ListAll(
	_ context.Context, caller entity.Principal, status *entity.TaskStatus, limit, offset int,
) ([]entity.Task, int, error) {
	f.gotCaller, f.gotStatus, f.gotLimit, f.gotOffset = caller, status, limit, offset

	return f.listed, len(f.listed), f.err
}

func (f *fakeTask) Create(context.Context, string, string, string) (entity.Task, error) {
	return f.stored, f.err
}

func (f *fakeTask) Get(context.Context, string, string) (entity.Task, error) {
	return f.stored, f.err
}

func (f *fakeTask) List(context.Context, string, *entity.TaskStatus, int, int) ([]entity.Task, int, error) {
	return []entity.Task{f.stored}, 1, f.err
}

func (f *fakeTask) Update(context.Context, string, string, string, string) (entity.Task, error) {
	return f.stored, f.err
}

func (f *fakeTask) Transition(context.Context, string, string, entity.TaskStatus) (entity.Task, error) {
	return f.stored, f.err
}

func (f *fakeTask) Delete(context.Context, string, string) error {
	return f.err
}

// newRouter serves the REST router over tk; the use cases a task test never reaches are nil.
func newRouter(cfg *config.Config, tk usecase.Task) http.Handler {
	return resttest.NewRouter(cfg, nil, nil, tk)
}
