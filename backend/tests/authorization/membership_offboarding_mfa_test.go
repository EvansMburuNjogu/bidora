package authorization_test

import (
	"context"
	"testing"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
)

type offboardingMFAControl struct {
	beginCalls, retargetCalls, cancelCalls int
}

func (c *offboardingMFAControl) BeginMembershipOffboarding(_ context.Context, input authzapp.MembershipOffboardingBegin) (authzapp.MembershipOffboardingOperation, error) {
	c.beginCalls++
	return authzapp.MembershipOffboardingOperation{ID: input.IDs.OperationID, State: "COMPLETED", Version: 1}, nil
}
func (*offboardingMFAControl) MarkMembershipTenantTransferred(context.Context, authzapp.ID, int64, int64, authzapp.ID, authzapp.OwnershipTransferResult, authzapp.MembershipOffboardingIDs, time.Time) (authzapp.MembershipOffboardingOperation, error) {
	return authzapp.MembershipOffboardingOperation{}, nil
}
func (*offboardingMFAControl) FinalizeMembershipOffboarding(context.Context, authzapp.ID, int64, authzapp.ID, authzapp.MembershipOffboardingIDs, time.Time) (authzapp.MembershipOffboardingOperation, error) {
	return authzapp.MembershipOffboardingOperation{}, nil
}
func (*offboardingMFAControl) GetMembershipOffboarding(context.Context, authzapp.ID, authzapp.ID, authzapp.ID, int64) (authzapp.MembershipOffboardingOperation, error) {
	return authzapp.MembershipOffboardingOperation{}, nil
}
func (c *offboardingMFAControl) RetargetMembershipOffboarding(_ context.Context, change authzapp.MembershipOffboardingChange) (authzapp.MembershipOffboardingOperation, error) {
	c.retargetCalls++
	return authzapp.MembershipOffboardingOperation{ID: change.OperationID, State: "STARTED", Version: change.ExpectedVersion + 1}, nil
}
func (c *offboardingMFAControl) CancelMembershipOffboarding(_ context.Context, change authzapp.MembershipOffboardingChange) (authzapp.MembershipOffboardingOperation, error) {
	c.cancelCalls++
	return authzapp.MembershipOffboardingOperation{ID: change.OperationID, State: "CANCELLED", Version: change.ExpectedVersion + 1}, nil
}

type offboardingMFATenant struct{ clearCalls int }

func (*offboardingMFATenant) TransferMembershipOwnership(context.Context, authzapp.MembershipOffboardingOperation, authzapp.ID, string, authzapp.ID, authzapp.MembershipOffboardingIDs, time.Time) (authzapp.OwnershipTransferResult, error) {
	return authzapp.OwnershipTransferResult{}, nil
}
func (t *offboardingMFATenant) ClearMembershipOwnershipFence(context.Context, authzapp.MembershipOffboardingOperation, time.Time) error {
	t.clearCalls++
	return nil
}

func TestMembershipOffboardingOperationsRequireRecentTOTPAAL2(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	type operation struct {
		name   string
		invoke func(*authzapp.MembershipOffboardingService, string, string, time.Time) error
		calls  func(*offboardingMFAControl) int
	}
	operations := []operation{
		{name: "begin", invoke: func(service *authzapp.MembershipOffboardingService, level, method string, assured time.Time) error {
			_, err := service.Begin(context.Background(), authzapp.MembershipOffboardingInput{ActorID: id(1), TenantID: id(2), TargetMembershipID: id(3), SuccessorMembershipID: id(4), CorrelationID: id(5), ActorContextGeneration: 1, ExpectedTargetGeneration: 2, Reason: "Ownership continuity", IdempotencyKey: "offboard-begin", SessionAuthenticatedAt: now, AssuranceAt: assured, AssuranceLevel: level, AssuranceMethod: method})
			return err
		}, calls: func(c *offboardingMFAControl) int { return c.beginCalls }},
		{name: "retarget", invoke: func(service *authzapp.MembershipOffboardingService, level, method string, assured time.Time) error {
			_, err := service.Retarget(context.Background(), authzapp.MembershipOffboardingChange{ActorID: id(1), TenantID: id(2), OperationID: id(6), SuccessorMembershipID: id(4), CorrelationID: id(5), ActorContextGeneration: 1, ExpectedVersion: 2, Reason: "Change successor", IdempotencyKey: "offboard-retarget", SessionAuthenticatedAt: now, AssuranceAt: assured, AssuranceLevel: level, AssuranceMethod: method})
			return err
		}, calls: func(c *offboardingMFAControl) int { return c.retargetCalls }},
		{name: "cancel", invoke: func(service *authzapp.MembershipOffboardingService, level, method string, assured time.Time) error {
			_, err := service.Cancel(context.Background(), authzapp.MembershipOffboardingChange{ActorID: id(1), TenantID: id(2), OperationID: id(6), CorrelationID: id(5), ActorContextGeneration: 1, ExpectedVersion: 2, Reason: "Cancel offboarding", IdempotencyKey: "offboard-cancel", SessionAuthenticatedAt: now, AssuranceAt: assured, AssuranceLevel: level, AssuranceMethod: method})
			return err
		}, calls: func(c *offboardingMFAControl) int { return c.cancelCalls }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			control, tenant := &offboardingMFAControl{}, &offboardingMFATenant{}
			service, err := authzapp.NewMembershipOffboardingService(control, tenant, &invitationIDs{}, invitationClock{at: now})
			if err != nil {
				t.Fatal(err)
			}
			for _, assurance := range []struct {
				level, method string
				at            time.Time
			}{
				{level: "AAL1", method: "PRIMARY_ONLY", at: now},
				{level: "AAL2", method: "RECOVERY_CODE", at: now},
				{level: "AAL2", method: "TOTP", at: now.Add(-16 * time.Minute)},
				{level: "AAL2", method: "TOTP", at: now.Add(time.Second)},
			} {
				if invokeErr := operation.invoke(service, assurance.level, assurance.method, assurance.at); invokeErr == nil || invokeErr.Error() != "membership_offboarding_invalid" {
					t.Fatalf("%s accepted insufficient assurance: %v", operation.name, invokeErr)
				}
			}
			if invokeErr := operation.invoke(service, "AAL2", "TOTP", now); invokeErr != nil {
				t.Fatal(invokeErr)
			}
			if operation.calls(control) != 1 {
				t.Fatal("offboarding assurance crossed control boundary", operation.name, operation.calls(control))
			}
			if operation.name == "cancel" && tenant.clearCalls != 1 {
				t.Fatal("allowed cancellation did not clear ownership fence")
			}
		})
	}
}
