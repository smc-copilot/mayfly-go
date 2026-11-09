package init

import (
	"testing"

	"mayfly-go/internal/pkg/config"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCompositionBuildsStandaloneServices(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	api := BuildDirectoryAPI(db, config.Aes{Key: "1234567890123456"})
	require.NotNil(t, api.ListService)
	require.NotNil(t, api.Connections)
	require.NotNil(t, api.Tester)
	require.NotNil(t, api.Syncer)
	require.NotNil(t, api.Directory)
	require.NotNil(t, api.Operations)
}
