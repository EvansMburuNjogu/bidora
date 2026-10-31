package application

import (
	"bidora/backend/internal/platformsettings/domain"
	"fmt"
)

// These Responses-compatible model IDs are selectable, not a claim of merchant
// access or provider readiness. The governed test verifies the exact model.
func SupportedOpenAIModels() []string {
	return []string{"gpt-4.1-mini", "gpt-4.1", "gpt-4o-mini", "gpt-4o"}
}
func SupportedOpenAIModel(model string) bool {
	for _, v := range SupportedOpenAIModels() {
		if model == v {
			return true
		}
	}
	return false
}

type SealedIntegrationSecret struct {
	ID                domain.ID
	Kind              string
	KeyVersion        int32
	Nonce, Ciphertext []byte
}

func (SealedIntegrationSecret) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED integration secret]")
}
