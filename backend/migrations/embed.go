// Package migrations exposes the reviewed SQL migration sets to the standalone
// migration command. Application runtimes do not import or execute this package.
package migrations

import "embed"

// Files contains every reviewed SQL migration. The migration command selects an
// allowlisted subdirectory and refuses targets other than its frozen target.
//
//go:embed control/*.sql tenant_common/*.sql tenant_tenderpreneur/*.sql
var Files embed.FS
