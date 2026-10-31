//go:build !darwin && !linux

package localbudget

import (
	"context"
	"errors"
)

func collectStorage(context.Context, []string) (Storage, error) {
	return Storage{}, errors.New("native storage collection unsupported on this platform")
}
