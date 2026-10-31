# Disaster recovery

Sources: [database §§13–14](../product/10_DATABASE_DESIGN.md), [operations §14](../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), OPS-002–006. Decision [013](27_ARCHITECTURE_DECISIONS.md). Execution: [deployment](../implementation/17_DEVOPS_DEPLOYMENT.md), [release](../implementation/19_RELEASE_READINESS.md).

## Recovery set

Back up control DB, each tenant DB, future exchange DB, immutable object versions, configuration/registry, secret/KMS recovery metadata and necessary event reconciliation state. Search is rebuildable from authorized sources; broker replay alone is not database backup. Encrypt backups, isolate credentials and verify recoverability of keys separately from ciphertext.

Source suggested RPOs are control/exchange ≤15 minutes, standard tenant ≤1 hour, enterprise ≤15 minutes or contract-specific. They are proposals until ADR-013 approval; RTO and plan targets remain unresolved. Do not advertise guarantees based on scheduled backup jobs alone.

## Restore-to-new procedure

1. Incident/change owner defines tenant/plane, restore point, allowed data loss and communication plan.
2. Preserve original evidence; isolate writes as approved. Restore into a new database/namespace with restricted credentials.
3. Verify schema/checksums, registry identity and actor/membership authority against current control state. Old restored membership projections cannot resurrect access.
4. Reconcile DB references to exact object versions, candidate/package hashes, audit facts, outbox/inbox and externally completed effects.
5. Rebuild authorized indexes; suppress stale/duplicate notifications and payment effects through operation IDs/reconciliation.
6. Run security and critical business smoke tests, including approval applicability and evidence retrieval.
7. Approve cutover; increment assignment generation, drain old pools, monitor; retain original target according to incident policy.

## Failure boundaries

Control and tenant backups may be at different points. Reconcile registry/membership/lifecycle rather than assume simultaneous restore. Orphan objects are not automatically garbage. Missing approved evidence blocks readiness and triggers incident handling. If future exchange accepted a submission after a bidder restore point, restore must recover acknowledgement from exchange—not resubmit automatically. KMS loss can make sealed bids irrecoverable even with valid DB/object backups; independent key recovery is mandatory before sealed release.

## Exercises and evidence

Source cadence proposes monthly sampled tenant restore and quarterly control/exchange exercises. Approve ownership/cadence before production. Measure actual restore duration, data loss, object integrity, unauthorized-access denial and replay safety. Record backup IDs, timestamps, schema versions, checksums, tests, approver and cutover decision. A successful backup API response is insufficient evidence.
