package scaffold_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entry is one member of a test archive.
type entry struct {
	name, body, link string
	typ              byte
	mode             int64
}

func targz(t *testing.T, entries ...entry) []byte {
	t.Helper()

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for i := range entries {
		e := &entries[i]

		typ, mode := e.typ, e.mode
		if typ == 0 {
			typ = tar.TypeReg
		}

		if mode == 0 {
			mode = 0o644
		}

		hdr := &tar.Header{Name: e.name, Typeflag: typ, Mode: mode, Linkname: e.link}

		switch typ {
		case tar.TypeXGlobalHeader:
			// git archive writes the commit here.
			hdr = &tar.Header{Typeflag: typ, PAXRecords: map[string]string{"comment": "5e4fc32"}}
		case tar.TypeReg:
			hdr.Size = int64(len(e.body))
		}

		require.NoError(t, tw.WriteHeader(hdr))

		if typ == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	return buf.Bytes()
}

// serve answers every request with the archive, as GitHub's archive endpoint does for a ref.
func serve(t *testing.T, archive []byte) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(archive); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)

	return srv.URL + "/archive/v0.1.0.tar.gz"
}

func fetcher() scaffold.Fetcher {
	return scaffold.Fetcher{Client: http.DefaultClient, MaxBytes: 1 << 20}
}

func TestArchiveURLPointsAtTheModulesRepository(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://github.com/acme/template/archive/v0.4.0.tar.gz",
		scaffold.ArchiveURL("github.com/acme/template", "v0.4.0"))
}

func TestFetchExtractsTheTemplateUnderItsTopDirectory(t *testing.T) {
	t.Parallel()

	url := serve(t, targz(t,
		entry{name: "pax_global_header", typ: tar.TypeXGlobalHeader},
		entry{name: "template-0.1.0/", typ: tar.TypeDir, mode: 0o755},
		entry{name: "template-0.1.0/scaffold/features.yaml", body: "module: x\n"},
		entry{name: "template-0.1.0/scripts/dev.sh", body: "#!/bin/sh\n", mode: 0o755},
		entry{name: "template-0.1.0/CLAUDE.md", body: "rules\n"},
		entry{name: "template-0.1.0/AGENTS.md", typ: tar.TypeSymlink, link: "CLAUDE.md"},
	))

	root, err := fetcher().Fetch(t.Context(), url, t.TempDir())
	require.NoError(t, err)

	assert.Equal(t, "template-0.1.0", filepath.Base(root))

	body, err := os.ReadFile(filepath.Join(root, "scaffold", "features.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "module: x\n", string(body))

	info, err := os.Stat(filepath.Join(root, "scripts", "dev.sh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())

	target, err := os.Readlink(filepath.Join(root, "AGENTS.md"))
	require.NoError(t, err)
	assert.Equal(t, "CLAUDE.md", target)
}

func TestFetchNamesTheStatusOfAFailedDownload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	_, err := fetcher().Fetch(t.Context(), srv.URL+"/archive/v9.9.9.tar.gz", t.TempDir())

	require.ErrorIs(t, err, scaffold.ErrFetch)
	assert.ErrorContains(t, err, "404")
	assert.ErrorContains(t, err, "v9.9.9")
}

func TestFetchRejectsUnsafeArchives(t *testing.T) {
	t.Parallel()

	manifest := entry{name: "t/scaffold/features.yaml", body: "module: x\n"}

	tests := []struct {
		name    string
		archive []entry
		reason  string
	}{
		{"a path out of the directory", []entry{manifest, {name: "t/../../evil", body: "x"}}, "outside"},
		{"an absolute path", []entry{manifest, {name: "/etc/evil", body: "x"}}, "outside"},
		{"a symlink out of the tree", []entry{manifest, {name: "t/a", typ: tar.TypeSymlink, link: "../../outside"}}, "outside"},
		{"an absolute symlink", []entry{manifest, {name: "t/a", typ: tar.TypeSymlink, link: "/etc/passwd"}}, "outside"},
		{"no template in it", []entry{{name: "t/README.md", body: "x"}}, scaffold.ManifestPath},
		{"two top directories", []entry{manifest, {name: "u/README.md", body: "x"}}, "one top directory"},
		{"more than MaxBytes", []entry{manifest, {name: "t/big", body: string(make([]byte, 2<<20))}}, "larger than"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			_, err := fetcher().Fetch(t.Context(), serve(t, targz(t, tt.archive...)), dir)

			require.ErrorIs(t, err, scaffold.ErrFetch)
			assert.ErrorContains(t, err, tt.reason)
			assert.NoFileExists(t, filepath.Join(filepath.Dir(dir), "evil"))
		})
	}
}

func TestFetchStopsWhenTheContextIsCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := fetcher().Fetch(ctx, serve(t, targz(t)), t.TempDir())

	require.ErrorIs(t, err, context.Canceled)
}
