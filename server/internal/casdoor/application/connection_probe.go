package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
	"mayfly-go/internal/casdoor/domain/repository"
	"mayfly-go/internal/casdoor/imsg"
	"mayfly-go/internal/pkg/consts"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/errorx"
)

type ProbeItem struct {
	Status     string `json:"status"`
	ReasonCode string `json:"reasonCode"`
}

type ConnectionProbeResult struct {
	Service ProbeItem `json:"service"`
	Apps    ProbeItem `json:"apps"`
	Users   ProbeItem `json:"users"`
}

type ConnectionTester struct {
	connections repository.Connections
	secrets     repository.Secrets
	access      *ScopeAuthorizer
	factory     directory.ProbeFactory
}

func NewConnectionTester(connections repository.Connections, secrets repository.Secrets, access *ScopeAuthorizer, factory directory.ProbeFactory) *ConnectionTester {
	return &ConnectionTester{connections: connections, secrets: secrets, access: access, factory: factory}
}

func (t *ConnectionTester) Test(ctx context.Context, input *entity.Connection, secret string) (*ConnectionProbeResult, error) {
	if t == nil || input == nil || t.connections == nil || t.secrets == nil || t.access == nil || t.factory == nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrConnectionInvalid)
	}
	candidate := *input
	candidate.Name = strings.TrimSpace(candidate.Name)
	candidate.Organization = strings.TrimSpace(candidate.Organization)
	candidate.ClientId = strings.TrimSpace(candidate.ClientId)
	candidate.ServerURL = strings.TrimRight(strings.TrimSpace(candidate.ServerURL), "/")
	if candidate.TimeoutSec == 0 {
		candidate.TimeoutSec = 15
	}
	if candidate.SyncIntervalSec == 0 {
		candidate.SyncIntervalSec = 3600
	}
	if !validConnection(&candidate) {
		return nil, errorx.NewBizI(ctx, imsg.ErrConnectionInvalid)
	}
	if strings.TrimSpace(secret) == "" {
		secret = ""
	}
	if candidate.Id == 0 {
		account := contextx.GetLoginAccount(ctx)
		if account == nil || account.Id != consts.AdminId {
			return nil, errorx.NewBizI(ctx, imsg.ErrConnectionNoAccess)
		}
	} else {
		if err := t.access.AssertConnectionAccess(ctx, candidate.Id, true); err != nil {
			return nil, err
		}
		saved, err := t.connections.GetConnection(ctx, candidate.Id)
		if err != nil {
			return nil, err
		}
		if saved == nil {
			return nil, errorx.NewBizI(ctx, imsg.ErrConnectionNotFound)
		}
		if strings.TrimRight(saved.ServerURL, "/") != candidate.ServerURL || saved.Organization != candidate.Organization {
			return nil, errorx.NewBizI(ctx, imsg.ErrConnectionTargetImmutable)
		}
		if saved.ClientId != candidate.ClientId && secret == "" {
			return nil, errorx.NewBizI(ctx, imsg.ErrConnectionSecretRequired)
		}
		if secret == "" {
			secret, err = t.secrets.Decrypt(saved.ClientSecretEnc)
			if err != nil {
				return nil, err
			}
		}
	}
	if secret == "" {
		return nil, errorx.NewBizI(ctx, imsg.ErrConnectionSecretRequired)
	}
	prober := t.factory.NewProbe(directory.ProbeConfig{
		ServerURL: candidate.ServerURL, Organization: candidate.Organization, AppOwner: candidate.AppOwner,
		ClientID: candidate.ClientId, ClientSecret: secret,
		Timeout: time.Duration(candidate.TimeoutSec) * time.Second, TLSVerify: candidate.TLSVerify != -1,
	})
	if prober == nil {
		return nil, errorx.NewBizI(ctx, imsg.ErrConnectionInvalid)
	}
	return &ConnectionProbeResult{
		Service: probeResult(prober.TestService(ctx)),
		Apps:    probeResult(prober.TestApps(ctx)),
		Users:   probeResult(prober.TestUsers(ctx)),
	}, nil
}

func probeResult(err error) ProbeItem {
	if err == nil {
		return ProbeItem{Status: "ok"}
	}
	reason := "probe_failed"
	var coded interface{ KindCode() string }
	if errors.As(err, &coded) {
		switch coded.KindCode() {
		case "unreachable", "timeout", "forbidden", "invalid_org", "bad_app_owner", "route_missing", "unexpected_payload", "http":
			reason = coded.KindCode()
		}
	}
	return ProbeItem{Status: "fail", ReasonCode: reason}
}
