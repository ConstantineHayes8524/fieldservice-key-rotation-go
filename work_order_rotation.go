package fieldkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type WorkOrder struct {
	ID                 string
	PhotoCount         int
	DispatchStatus     string
	TechnicianFollowUp string
}

type RotationDecision struct {
	GraceHours int
	Action     string
}

type RotationReport struct {
	WorkOrder      WorkOrder
	Decision       RotationDecision
	CreatedKey     json.RawMessage
	RotatedKey     json.RawMessage
	DeploymentLogs json.RawMessage
}

type createdKey struct {
	ID    string `json:"id"`
	KeyID string `json:"key_id"`
	Key   string `json:"key"`
}

func DecideGrace(deploymentsUsingOldValue int) RotationDecision {
	if deploymentsUsingOldValue > 0 {
		return RotationDecision{GraceHours: 24, Action: "keep old key active while deployments update"}
	}
	return RotationDecision{GraceHours: 1, Action: "finish rotation after final deployment check"}
}

func RotateForWorkOrder(ctx context.Context, client *Client, order WorkOrder, deploymentsUsingOldValue int) (report RotationReport, runErr error) {
	if order.ID == "" {
		return RotationReport{}, fmt.Errorf("work order is required")
	}
	decision := DecideGrace(deploymentsUsingOldValue)
	stamp := time.Now().UTC().Format("20060102T150405Z")
	created, err := client.CreateTemporaryKey(ctx, "fieldservice-rotation-"+order.ID, "create-"+stamp)
	if err != nil {
		return RotationReport{}, err
	}
	var temporaryKey createdKey
	if err := json.Unmarshal(created, &temporaryKey); err != nil {
		return RotationReport{}, fmt.Errorf("created temporary key has no id")
	}
	if temporaryKey.KeyID != "" {
		temporaryKey.ID = temporaryKey.KeyID
	}
	if temporaryKey.ID == "" {
		return RotationReport{}, fmt.Errorf("created temporary key has no id")
	}
	var rotatedKeyID string
	defer func() {
		// Cleanup must still run when the work-order context has been canceled.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := client.RevokeKey(cleanupCtx, temporaryKey.ID); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("revoke temporary key %s: %w", temporaryKey.ID, err))
		}
		if rotatedKeyID != "" && rotatedKeyID != temporaryKey.ID {
			if err := client.RevokeKey(cleanupCtx, rotatedKeyID); err != nil {
				runErr = errors.Join(runErr, fmt.Errorf("revoke rotated key %s: %w", rotatedKeyID, err))
			}
		}
	}()
	rotated, err := client.RotateKey(ctx, temporaryKey.ID, "rotate-"+stamp, decision.GraceHours)
	if err != nil {
		return RotationReport{}, err
	}
	var rotatedKey createdKey
	if err := json.Unmarshal(rotated, &rotatedKey); err != nil {
		return RotationReport{}, fmt.Errorf("decode rotated key: %w", err)
	}
	rotatedKeyID = rotatedKey.Key
	if len(rotatedKeyID) >= 4 {
		rotatedKeyID = "ifr_..." + rotatedKeyID[len(rotatedKeyID)-4:]
	} else {
		rotatedKeyID = rotatedKey.KeyID
	}
	logs, err := client.SearchLogs(ctx, "deployment old API value work-order "+order.ID)
	if err != nil {
		return RotationReport{}, err
	}
	return RotationReport{WorkOrder: order, Decision: decision, CreatedKey: created, RotatedKey: rotated, DeploymentLogs: logs}, nil
}
