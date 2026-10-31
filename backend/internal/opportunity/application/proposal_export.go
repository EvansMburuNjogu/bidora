package application

import (
	"context"
	"crypto/sha256"
	"errors"
)

var ErrProposalExportUnsupported = errors.New("proposal_export_unsupported")
var ErrProposalExportTooLarge = errors.New("proposal_export_too_large")

type ProposalRenderer interface {
	Render(string, ProposalDocument) ([]byte, error)
}
type ProposalExport struct {
	Bytes                  []byte
	SHA256                 [32]byte
	ContentType, Extension string
	Revision               int64
}
type ProposalExportService struct {
	auth     TechnicalResponseAuthorizer
	store    ProposalStore
	renderer ProposalRenderer
}

func NewProposalExportService(auth TechnicalResponseAuthorizer, store ProposalStore, renderer ProposalRenderer) (*ProposalExportService, error) {
	if auth == nil || store == nil || renderer == nil {
		return nil, errors.New("proposal_export_service_invalid")
	}
	return &ProposalExportService{auth, store, renderer}, nil
}
func (s *ProposalExportService) Export(ctx context.Context, subject WorkflowSubject, bidID, versionID ID, format string) (ProposalExport, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || versionID.IsZero() || (format != "pdf" && format != "docx") {
		return ProposalExport{}, safe("proposal_export_invalid", nil)
	}
	authority, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return ProposalExport{}, safe("proposal_not_authorized", err)
	}
	proposal, err := s.store.GetProposalVersion(ctx, authority, bidID, versionID)
	if errors.Is(err, ErrProposalNotFound) {
		return ProposalExport{}, safe("proposal_not_found", err)
	}
	if err != nil {
		return ProposalExport{}, safe("proposal_export_unavailable", err)
	}
	content, err := s.renderer.Render(format, proposal)
	if errors.Is(err, ErrProposalExportUnsupported) {
		return ProposalExport{}, safe("proposal_export_unsupported", err)
	}
	if errors.Is(err, ErrProposalExportTooLarge) {
		return ProposalExport{}, safe("proposal_export_too_large", err)
	}
	if err != nil {
		return ProposalExport{}, safe("proposal_export_unavailable", err)
	}
	result := ProposalExport{Bytes: content, SHA256: sha256.Sum256(content), Revision: proposal.Revision}
	if format == "pdf" {
		result.ContentType = "application/pdf"
		result.Extension = "pdf"
	} else {
		result.ContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		result.Extension = "docx"
	}
	return result, nil
}
