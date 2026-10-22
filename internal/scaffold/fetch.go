package scaffold

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrFetch is a template source archive that could not be downloaded or is not safe to extract.
var ErrFetch = errors.New("cannot fetch the template")

// ArchiveURL is the source archive of ref (a tag, branch or commit) in the GitHub repository
// that serves module.
func ArchiveURL(module, ref string) string {
	return "https://" + module + "/archive/" + ref + ".tar.gz"
}

// Fetcher downloads a template's source archive.
type Fetcher struct {
	Client *http.Client
	// MaxBytes caps the archive's extracted size.
	MaxBytes int64
}

// Fetch downloads the .tar.gz at url, extracts it into dir and returns the template's root: the
// archive's one top directory, which must hold the scaffold manifest.
func (f Fetcher) Fetch(ctx context.Context, url, dir string) (string, error) {
	ref := strings.TrimSuffix(path.Base(url), ".tar.gz")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("scaffold - Fetch - http.NewRequestWithContext: %w", err)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("scaffold - Fetch - %s: %w", ref, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w %s: %s answered %s", ErrFetch, ref, url, resp.Status)
	}

	if err := f.extract(resp.Body, dir); err != nil {
		return "", fmt.Errorf("%w %s: %w", ErrFetch, ref, err)
	}

	return templateRoot(dir)
}

// extract writes the archive's directories, regular files and symlinks into dir.
func (f Fetcher) extract(body io.Reader, dir string) error {
	gz, err := gzip.NewReader(body)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}

	tr := tar.NewReader(&limitReader{r: gz, left: f.MaxBytes})

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		if err := writeEntry(tr, hdr, dir); err != nil {
			return err
		}
	}
}

func writeEntry(tr *tar.Reader, hdr *tar.Header, dir string) error {
	switch hdr.Typeflag {
	case tar.TypeDir, tar.TypeReg, tar.TypeSymlink:
	default:
		// pax headers and anything git never archives.
		return nil
	}

	name := path.Clean(hdr.Name)
	if !filepath.IsLocal(name) {
		return fmt.Errorf("%s %w", hdr.Name, errOutsideArchive)
	}

	dst := filepath.Join(dir, filepath.FromSlash(name))

	switch hdr.Typeflag {
	case tar.TypeDir:
		return mkdir(dst)
	case tar.TypeSymlink:
		return writeSymlink(name, hdr.Linkname, dst)
	default:
		return writeFile(tr, dst, hdr.FileInfo().Mode().Perm())
	}
}

func mkdir(dst string) error {
	if err := os.MkdirAll(dst, _dirPerm); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	return nil
}

func writeSymlink(name, target, dst string) error {
	if path.IsAbs(target) || !filepath.IsLocal(path.Join(path.Dir(name), target)) {
		return fmt.Errorf("%s → %s %w", name, target, errOutsideArchive)
	}

	if err := mkdir(filepath.Dir(dst)); err != nil {
		return err
	}

	if err := os.Symlink(target, dst); err != nil {
		return fmt.Errorf("os.Symlink: %w", err)
	}

	return nil
}

func writeFile(r io.Reader, dst string, perm os.FileMode) error {
	if err := mkdir(filepath.Dir(dst)); err != nil {
		return err
	}

	file, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("os.OpenFile: %w", err)
	}

	if _, err := io.Copy(file, r); err != nil {
		_ = file.Close()

		return fmt.Errorf("io.Copy: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("file.Close: %w", err)
	}

	// The umask may have narrowed perm.
	if err := os.Chmod(dst, perm); err != nil {
		return fmt.Errorf("os.Chmod: %w", err)
	}

	return nil
}

// templateRoot is dir's one top directory, which must hold the scaffold manifest.
func templateRoot(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("scaffold - Fetch - os.ReadDir: %w", err)
	}

	if len(entries) != 1 || !entries[0].IsDir() {
		return "", fmt.Errorf("%w: the archive must have one top directory", ErrFetch)
	}

	root := filepath.Join(dir, entries[0].Name())

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ManifestPath))); err != nil {
		return "", fmt.Errorf("%w: the archive has no %s", ErrFetch, ManifestPath)
	}

	return root, nil
}

// limitReader fails once more than left bytes are read, unlike io.LimitReader, which ends quietly.
type limitReader struct {
	r    io.Reader
	left int64
}

func (l *limitReader) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)

	l.left -= int64(n)
	if l.left < 0 {
		return n, errArchiveTooLarge
	}

	return n, err //nolint:wrapcheck // io.Reader contract: io.EOF must pass through unwrapped.
}

var (
	errArchiveTooLarge = errors.New("the archive is larger than the size limit")
	errOutsideArchive  = errors.New("points outside the archive")
)
