package localprofile

import (
	"errors"
	"strconv"
)

// Load parses explicit local-profile settings using the supplied environment
// reader. It does not read the process environment itself, connect to a service,
// or enable the profile in the API. On any error it returns an empty Config.
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("environment reader is required")
	}
	c := Config{
		Environment: getenv("BIDORA_ENV"),
		Profile:     getenv("BIDORA_PROFILE"),
	}
	var err error
	if c.CPUs, err = requiredInt(getenv("BIDORA_LOCAL_CPUS"), "BIDORA_LOCAL_CPUS"); err != nil {
		return Config{}, err
	}
	if c.MemoryMiB, err = requiredInt(getenv("BIDORA_LOCAL_MEMORY_MIB"), "BIDORA_LOCAL_MEMORY_MIB"); err != nil {
		return Config{}, err
	}
	if c.StorageGiB, err = requiredInt(getenv("BIDORA_LOCAL_STORAGE_GIB"), "BIDORA_LOCAL_STORAGE_GIB"); err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// requiredInt accepts canonical positive ASCII decimal integers. Bound lexical
// work before conversion and replace strconv errors so values never leak.
func requiredInt(raw, field string) (int, error) {
	invalid := func() (int, error) {
		return 0, errors.New(field + " must be an explicit positive decimal integer")
	}
	if len(raw) == 0 || len(raw) > 20 || raw[0] < '1' || raw[0] > '9' {
		return invalid()
	}
	for i := 1; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return invalid()
		}
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return invalid()
	}
	return n, nil
}
