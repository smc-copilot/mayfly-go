package application

import (
	"encoding/json"
	"fmt"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/entity"
)

const langfuseVerificationProtocol = 1

// The proof is meaningful only for this exact saved revision and the current
// authenticated-project verification protocol. Legacy health-only ready flags
// cannot establish that two configured instances have different projects.
type langfuseProjectProof struct {
	Protocol  int    `json:"protocol"`
	Revision  int64  `json:"revision"`
	ProjectId string `json:"projectId"`
}

func verifiedLangfuseProject(row *entity.OctopReadConnection) (string, bool) {
	if row.VerifyState != entity.VerifyStateReady {
		return "", false
	}
	var extra struct {
		Proof *langfuseProjectProof `json:"langfuseVerification"`
	}
	if err := json.Unmarshal([]byte(row.ExtraJSON), &extra); err != nil {
		return "", false
	}
	proof := extra.Proof
	if proof == nil || proof.Protocol != langfuseVerificationProtocol || proof.Revision != row.Revision || proof.ProjectId == "" {
		return "", false
	}
	if row.ProjectId != "" && row.ProjectId != proof.ProjectId {
		return "", false
	}
	return proof.ProjectId, true
}

func langfuseVerifiedExtra(row *entity.OctopReadConnection, res *dto.ReadConnectionTestResult) (string, error) {
	extra := map[string]json.RawMessage{}
	if row.ExtraJSON != "" {
		if err := json.Unmarshal([]byte(row.ExtraJSON), &extra); err != nil {
			return "", fmt.Errorf("decode read connection verification metadata: %w", err)
		}
		if extra == nil {
			extra = map[string]json.RawMessage{}
		}
	}
	proof, err := json.Marshal(langfuseProjectProof{Protocol: langfuseVerificationProtocol, Revision: row.Revision, ProjectId: res.ProjectId})
	if err != nil {
		return "", err
	}
	extra["langfuseVerification"] = proof
	encoded, err := json.Marshal(extra)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
