package domain

import (
	"crypto/sha256"
	"encoding/json"
)

const PesaPal Kind = "PESAPAL"
const PesaPalSandboxAPI = "https://cybqa.pesapal.com/pesapalv3/api"
const PesaPalLiveAPI = "https://pay.pesapal.com/v3/api"

type PesaPalConfiguration struct {
	Enabled                                         bool
	Environment, MerchantName, CredentialsSecretRef string
	NotificationID                                  ID
}

func (c PesaPalConfiguration) API() string {
	if c.Environment == "SANDBOX" {
		return PesaPalSandboxAPI
	}
	if c.Environment == "LIVE" {
		return PesaPalLiveAPI
	}
	return ""
}
func (c PesaPalConfiguration) valid() bool {
	return (c.Environment == "SANDBOX" || c.Environment == "LIVE") && safeText(c.MerchantName, 1, 120) && secretRef(c.CredentialsSecretRef) && (!c.Enabled || !c.NotificationID.IsZero() && c.CredentialsSecretRef != "unconfigured")
}
func NewPesaPalConfiguration(p PesaPalConfiguration) (Configuration, [32]byte, error) {
	c := Configuration{Kind: PesaPal, PesaPal: &p}
	if !c.Valid() {
		return Configuration{}, [32]byte{}, ErrInvalid
	}
	raw, e := json.Marshal(struct {
		Kind    Kind
		PesaPal *PesaPalConfiguration
	}{PesaPal, &p})
	if e != nil {
		return Configuration{}, [32]byte{}, ErrInvalid
	}
	return c, sha256.Sum256(raw), nil
}
func (c Configuration) Hash() ([32]byte, error) {
	if c.Kind == PesaPal && c.PesaPal != nil {
		_, h, e := NewPesaPalConfiguration(*c.PesaPal)
		return h, e
	}
	_, h, e := NewConfiguration(c.Kind, c.SMTP, c.OpenAI)
	return h, e
}
