package orders

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// toyQty takes an ASCII line of a toy and returns the
// quantity of toys to make.
//
// Ex:
//
//	line:= []byte("10x toy car")
//	toyQty(line) => 10
func toyQty(in []byte) (int, error) {
	parts := bytes.Split(in, []byte{'x'})
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid request %q", in)
	}
	rawQty := parts[0]
	qty, err := strconv.Atoi(string(rawQty))
	if err != nil {
		return 0, err
	}
	return qty, nil
}

// MostCopies takes and ASCII line of a comma-separated
// list of toys to make and returns the toy with the
// most copies to make.
//
// Ex:
//
//	line:= []byte("10x toy car,15x dog on a string,4x inflatable motorcycle\n"
//	MostCopies(line) => []byte("15x dog on a string\n")
func MostCopies(in []byte) ([]byte, error) {
	toys := bytes.Split(in, []byte{','})
	if len(toys) == 0 {
		return []byte{}, nil
	}
	errs := []string{}
	max := slices.MaxFunc(toys, func(a, b []byte) int {
		aQty, aErr := toyQty(a)
		if aErr != nil {
			errs = append(errs, aErr.Error())
		}

		bQty, bErr := toyQty(b)
		if bErr != nil {
			errs = append(errs, bErr.Error())
		}
		return aQty - bQty
	})

	if len(errs) > 0 {
		return []byte{}, errors.New(strings.Join(errs, ","))
	}
	return max, nil
}
