package response

import "github.com/m7medvision/go-clean-template/internal/entity"

// TaskList -.
type TaskList struct {
	Tasks []entity.Task `json:"tasks"`
	Total int           `json:"total" example:"42"`
}
