package orders

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func toyQty(in string) (int, error) {
	parts := strings.Split(in, "x")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid request %q", in)
	}
	rawQty := parts[0]
	qty, err := strconv.Atoi(rawQty)
	if err != nil {
		return 0, err
	}
	return qty, nil
}

func MostCopies(in string) (string, error) {
	toys := strings.Split(in, ",")
	if len(toys) == 0 {
		return "", nil
	}
	errs := []string{}
	max := slices.MaxFunc(toys, func(a, b string) int {
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
		return "", errors.New(strings.Join(errs, ","))
	}
	return max, nil
}
