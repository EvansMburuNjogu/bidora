package pilot

import (
	"bidora/backend/internal/platformsettings/pesapalruntime"
	"errors"
	"strconv"
)

// Credentials/enabled/environment/IPN revision are managed in encrypted settings.
// Only the fixed public callback and explicit production-operation gate belong
// to deployment. Empty deployment configuration leaves provider use unavailable.
func LoadPesaPalDeploymentOptions(getenv func(string) string) (pesapalruntime.Options, error) {
	o := pesapalruntime.Options{CallbackURL: getenv("BIDORA_PESAPAL_CALLBACK_URL")}
	raw := getenv("BIDORA_PESAPAL_ALLOW_LIVE")
	if raw != "" {
		v, e := strconv.ParseBool(raw)
		if e != nil {
			return o, errors.New("pesapal_deployment_invalid")
		}
		o.AllowLive = v
	}
	if !o.Valid() {
		return o, errors.New("pesapal_deployment_invalid")
	}
	return o, nil
}
