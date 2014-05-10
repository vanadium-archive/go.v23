package build

import (
	"fmt"
	"math/big"
)

// bigCmplx reprepresents a constant complex number.
type bigCmplx struct {
	re, im *big.Rat
}

func (bc bigCmplx) Equal(other bigCmplx) bool {
	return bc.re.Cmp(other.re) == 0 && bc.im.Cmp(other.im) == 0
}

func (bc bigCmplx) Add(other bigCmplx) bigCmplx {
	return bigCmplx{
		re: new(big.Rat).Add(bc.re, other.re),
		im: new(big.Rat).Add(bc.im, other.im),
	}
}

func (bc bigCmplx) Sub(other bigCmplx) bigCmplx {
	return bigCmplx{
		re: new(big.Rat).Sub(bc.re, other.re),
		im: new(big.Rat).Sub(bc.im, other.im),
	}
}

func (bc bigCmplx) Mul(other bigCmplx) bigCmplx {
	rr := new(big.Rat).Mul(bc.re, other.re)
	ri := new(big.Rat).Mul(bc.re, other.im)
	ir := new(big.Rat).Mul(bc.im, other.re)
	ii := new(big.Rat).Mul(bc.im, other.im)

	return bigCmplx{
		re: new(big.Rat).Sub(rr, ii),
		im: new(big.Rat).Add(ir, ri),
	}
}

func (bc bigCmplx) Div(other bigCmplx) (bigCmplx, error) {
	// (a+ib)/(x+iy) = (a+ib)(x-iy)/(x*x+y*y)
	a := bc.re
	b := bc.im
	x := other.re
	y := other.im

	ax := new(big.Rat).Mul(a, x)
	ay := new(big.Rat).Mul(a, y)
	bx := new(big.Rat).Mul(b, x)
	by := new(big.Rat).Mul(b, y)

	// 1/(x*x+y*y)
	xx := new(big.Rat).Mul(x, x)
	yy := new(big.Rat).Mul(y, y)
	xxyy := new(big.Rat).Add(xx, yy)
	if xxyy.Cmp(bigRatZero) == 0 {
		return bigCmplx{}, fmt.Errorf("cannot divide by zero")
	}

	re := new(big.Rat).Add(ax, by)
	im := new(big.Rat).Sub(bx, ay)

	return bigCmplx{
		re: new(big.Rat).Quo(re, xxyy),
		im: new(big.Rat).Quo(im, xxyy),
	}, nil
}
