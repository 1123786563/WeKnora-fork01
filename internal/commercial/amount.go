package commercial

import (
	"errors"
	"math"
	"math/big"
	"regexp"
)

// Credits is measured in millionths of a Credit.
type Credits int64

// CNYFen is an amount of Chinese yuan represented in fen.
type CNYFen int64

var decimalCredits = regexp.MustCompile(`^(0|[0-9]+)(\.[0-9]+)?$`)

// ParseCredits parses a non-negative decimal Credit amount with six-place precision.
func ParseCredits(input string) (Credits, error) {
	if !decimalCredits.MatchString(input) {
		return 0, errors.New("credits must be a non-negative decimal")
	}
	r, ok := new(big.Rat).SetString(input)
	if !ok {
		return 0, errors.New("invalid credits")
	}
	scaled := new(big.Rat).Mul(r, big.NewRat(1_000_000, 1))
	if scaled.Denom().Cmp(big.NewInt(1)) != 0 || scaled.Num().Sign() < 0 || scaled.Num().Cmp(big.NewInt(math.MaxInt64)) > 0 {
		return 0, errors.New("credits must have at most six decimal places and fit int64")
	}
	return Credits(scaled.Num().Int64()), nil
}

func (c Credits) String() string {
	return formatFixed(int64(c), 6)
}

func (f CNYFen) String() string {
	return formatFixed(int64(f), 2)
}

func formatFixed(value int64, places int) string {
	negative := value < 0
	var magnitude uint64
	if negative {
		magnitude = uint64(-(value + 1)) + 1
	} else {
		magnitude = uint64(value)
	}
	pow := uint64(1)
	for i := 0; i < places; i++ {
		pow *= 10
	}
	whole, fraction := magnitude/pow, magnitude%pow
	result := big.NewInt(int64(whole)).String()
	if negative {
		result = "-" + result
	}
	return result + "." + leftPad(fraction, places)
}

func leftPad(value uint64, places int) string {
	s := big.NewInt(int64(value)).String()
	for len(s) < places {
		s = "0" + s
	}
	return s
}
