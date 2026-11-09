package persistence

import (
	"context"
	"errors"

	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/pkg/contextx"

	"gorm.io/gorm"
)

type connectionRepository struct {
	db *gorm.DB
}

func NewConnectionRepository(db *gorm.DB) repository.Connections {
	return &connectionRepository{db: db}
}

func (r *connectionRepository) GetConnection(ctx context.Context, id uint64) (*entity.Connection, error) {
	var conn entity.Connection
	err := r.db.WithContext(ctx).Where("id = ? AND is_deleted = 0", id).First(&conn).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

func (r *connectionRepository) SaveConnection(ctx context.Context, conn *entity.Connection) error {
	if conn.Id == 0 {
		conn.FillBaseInfo(contextx.GetLoginAccount(ctx))
		return r.db.WithContext(ctx).Create(conn).Error
	}
	conn.FillBaseInfo(contextx.GetLoginAccount(ctx))
	result := r.db.WithContext(ctx).Model(&entity.Connection{}).
		Where("id = ? AND is_deleted = 0", conn.Id).
		Updates(map[string]any{
			"name": conn.Name, "server_url": conn.ServerURL, "organization": conn.Organization,
			"app_owner": conn.AppOwner, "client_id": conn.ClientId, "client_secret_enc": conn.ClientSecretEnc,
			"timeout_sec": conn.TimeoutSec, "sync_interval_sec": conn.SyncIntervalSec, "tls_verify": conn.TLSVerify,
			"management_enabled": conn.ManagementEnabled, "credential_configured": conn.CredentialConfigured,
			"credential_rotated_at": conn.CredentialRotatedAt,
			"update_time":           conn.UpdateTime, "modifier_id": conn.ModifierId, "modifier": conn.Modifier,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.WithContext(ctx).Model(&entity.Connection{}).
			Where("id = ? AND is_deleted = 0", conn.Id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}
