package storage

import (
	"os"
	"testing"

	"github.com/ellis-chen/fa/internal"
	"github.com/stretchr/testify/require"
)

func TestCreateUploadManager(t *testing.T) {
	os.RemoveAll("foo.db")

	db := createStoreManager(&internal.Config{Dbname: "foo.db"})
	_, _ = db.LoadTrialAppApply("g1")

	_ = db.AddCopyLineG("a.txt", -1, "b/", 2, "g1")

	err := db.NewBChore(&BackupChore{Path: "/ellis-chen/users"})
	require.NoError(t, err)
	chore := db.QueryLatestChore()
	require.NotNil(t, chore)
	require.Equal(t, "/ellis-chen/users", chore.Path)

	err = db.ChgBChoreState(chore.ID, ChoreStateACKED)
	require.NoError(t, err)
	chore = db.QueryLatestChore()
	require.Equal(t, ChoreStateACKED, chore.State)
}
