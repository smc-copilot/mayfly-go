package application

import (
	"context"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/pkg/consts"
	"mayfly-go/pkg/logx"
	"mayfly-go/pkg/taskx"
	"sync"
	"time"
)

func (s *ServiceManager) StartScheduler() error {
	var running sync.Mutex
	return taskx.BindCronTaskWithLock("casdoor-service-directory", "0 * * * * *", 9*time.Minute, true, func() {
		if !running.TryLock() {
			return
		}
		defer running.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer cancel()
		s.syncDue(ctx)
	})
}
func (s *ServiceManager) syncDue(ctx context.Context) {
	// Internal scheduled reads deliberately do not fabricate an interactive account.
	services, err := s.store.List(ctx, consts.AdminId)
	if err != nil {
		logx.Error("casdoor service enumeration failed")
		return
	}
	for _, service := range services {
		if ctx.Err() != nil {
			return
		}
		interval := time.Duration(max(service.SyncIntervalSec, 60)) * time.Second
		if service.VerifiedAt == nil || time.Since(*service.VerifiedAt) >= interval {
			svc, cfg, err := s.config(ctx, service.Id)
			if err != nil {
				logx.Error("casdoor scheduled configuration failed")
				continue
			}
			orgs, err := s.gateway.Validate(ctx, cfg)
			if err != nil {
				if saveErr := s.store.Failure(ctx, service.Id, svc.CredentialRevision, "management_probe_failed"); saveErr != nil {
					logx.Error("casdoor service status update failed")
				}
				continue
			}
			if err := s.store.Discover(ctx, svc, orgs); err != nil {
				logx.Error("casdoor organization discovery save failed")
				continue
			}
		}
		orgs, err := s.store.Organizations(ctx, service.Id)
		if err != nil {
			logx.Error("casdoor scheduled organization query failed")
			continue
		}
		for _, org := range orgs {
			conn, err := s.deps.Connections.GetConnection(ctx, org.ConnectionID)
			if err != nil {
				logx.Error("casdoor scheduled context query failed")
				continue
			}
			if !organizationDue(conn, interval, time.Now()) {
				continue
			}
			if _, err := s.syncOrganization(ctx, &org); err != nil {
				logx.Error("casdoor scheduled organization synchronization failed")
			}
		}
	}
}
func organizationDue(conn *entity.Connection, interval time.Duration, now time.Time) bool {
	if conn == nil {
		return false
	}
	return conn.AppSyncAt == nil || conn.UserSyncAt == nil || now.Sub(*conn.AppSyncAt) >= interval || now.Sub(*conn.UserSyncAt) >= interval
}
