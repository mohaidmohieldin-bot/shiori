package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/go-shiori/shiori/internal/database"
	"github.com/go-shiori/shiori/internal/model"
	"github.com/stretchr/testify/require"
)

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()

	fn()

	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

func Test_printHandler_JSONIncludesFileFlags(t *testing.T) {
	// Force the SQLite database inside the data directory.
	t.Setenv("SHIORI_DATABASE_URL", "")
	t.Setenv("SHIORI_DBMS", "")

	ctx := context.Background()
	dataDir := t.TempDir()

	db, err := database.OpenSQLiteDatabase(ctx, filepath.Join(dataDir, "shiori.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))

	saved, err := db.SaveBookmarks(ctx, true,
		model.BookmarkDTO{Bookmark: model.Bookmark{URL: "https://example.com/with-files", Title: "With files"}},
		model.BookmarkDTO{Bookmark: model.Bookmark{URL: "https://example.com/without-files", Title: "Without files"}},
	)
	require.NoError(t, err)
	require.Len(t, saved, 2)
	withFiles, withoutFiles := saved[0], saved[1]

	// Create archive, ebook and thumbnail files for the first bookmark only.
	for _, rel := range []string{
		model.GetArchivePath(&withFiles),
		model.GetEbookPath(&withFiles),
		model.GetThumbnailPath(&withFiles),
	} {
		path := filepath.Join(dataDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("test"), 0o644))
	}

	cmd := ShioriCmd()
	cmd.SetArgs([]string{"--storage-directory", dataDir, "print", "--json"})
	output := captureStdout(t, func() {
		require.NoError(t, cmd.ExecuteContext(ctx))
	})

	var printed []model.BookmarkDTO
	require.NoError(t, json.Unmarshal([]byte(output), &printed), "output: %s", output)
	require.Len(t, printed, 2)

	byID := map[int]model.BookmarkDTO{}
	for _, b := range printed {
		byID[b.ID] = b
	}

	got := byID[withFiles.ID]
	require.True(t, got.HasArchive, "HasArchive should be true when the archive file exists")
	require.True(t, got.HasEbook, "HasEbook should be true when the ebook file exists")
	require.True(t, got.HasThumbnail, "HasThumbnail should be true when the thumbnail file exists")
	require.Equal(t, "/bookmark/"+strconv.Itoa(withFiles.ID)+"/thumb", got.ImageURL)

	got = byID[withoutFiles.ID]
	require.False(t, got.HasArchive)
	require.False(t, got.HasEbook)
	require.False(t, got.HasThumbnail)
	require.Empty(t, got.ImageURL)
}
