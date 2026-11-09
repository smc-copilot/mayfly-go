package persistence

import (
	"context"
	"testing"
	"time"

	"mayfly-go/internal/casdoor/domain/entity"

	"github.com/stretchr/testify/require"
)

func TestOperationsListExcludesPreviewsAndOtherConnections(t *testing.T) {
	db := snapshotTestDB(t)
	require.NoError(t, db.AutoMigrate(&entity.Change{}))
	now := time.Now()
	for _, row := range []entity.Change{
		{ConnectionId: 1, ActorId: 1, Action: "user.update", Status: "verified", SubmittedAt: &now},
		{ConnectionId: 1, ActorId: 1, Action: "user.update", Status: "pending"},
		{ConnectionId: 2, ActorId: 1, Action: "user.update", Status: "verified", SubmittedAt: &now},
	} {
		row.FillBaseInfo(nil)
		require.NoError(t, db.Create(&row).Error)
	}
	rows, total, err := NewOperationQuery(db).ListOperations(context.Background(), 1, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, "verified", rows[0].Status)
}
