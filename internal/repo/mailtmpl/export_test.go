package mailtmpl

import (
	"io/fs"

	"github.com/m7medvision/go-clean-template/internal/repo"
)

// Load builds a Renderer from fsys, laid out like the embedded templates/.
func Load(fsys fs.FS) (repo.MailRenderer, error) {
	return load(fsys)
}

// TemplateNames lists the emails the embedded templates/ defines.
func TemplateNames() ([]string, error) {
	return emailNames(_templates)
}
