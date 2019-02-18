package helpers

import (
	"errors"
	"math/big"
)

func BigAddStrings(x string, y string) (string, error) {
	err := errors.New("error parse string to big.Int")
	xAmount, ok := new(big.Int).SetString(x, 10)
	if !ok {
		return "", err
	}
	result := new(big.Int)
	result, ok = result.SetString(y, 10)
	if !ok {
		return "", err
	}
	result.Add(result, xAmount)
	return result.String(), nil
}
