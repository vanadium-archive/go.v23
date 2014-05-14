package compile

import (
	"math/big"
)

// bigComplex represents a constant complex number.  The semantics are similar
// to big.Rat; methods are typically of the form:
//   func (z *Int) Op(x, y *Int) *Int
// and implement operations z = x Op y with the result as receiver.

type bigComplex struct {
	re, im *big.Rat
}

func makeComplex() bigComplex {
	return bigComplex{new(big.Rat), new(big.Rat)}
}

// realComplex returns a bigComplex with real part re, and imaginary part zero.
func realComplex(re *big.Rat) bigComplex {
	return bigComplex{re: re, im: bigRatZero}
}

func (z bigComplex) Equal(x bigComplex) bool {
	return z.re.Cmp(x.re) == 0 && z.im.Cmp(x.im) == 0
}

func (z bigComplex) Add(x, y bigComplex) bigComplex {
	z.re.Add(x.re, y.re)
	z.im.Add(x.im, y.im)
	return z
}

func (z bigComplex) Sub(x, y bigComplex) bigComplex {
	z.re.Sub(x.re, y.re)
	z.im.Sub(x.im, y.im)
	return z
}

func (z bigComplex) Neg(x bigComplex) bigComplex {
	z.re.Neg(x.re)
	z.im.Neg(x.im)
	return z
}

func (z bigComplex) Mul(x, y bigComplex) bigComplex {
	// (a+bi) * (c+di) = (ac-bd) + (bc+ad)i
	ac := new(big.Rat).Mul(x.re, y.re)
	ad := new(big.Rat).Mul(x.re, y.im)
	bc := new(big.Rat).Mul(x.im, y.re)
	bd := new(big.Rat).Mul(x.im, y.im)
	z.re.Sub(ac, bd)
	z.im.Add(bc, ad)
	return z
}

func (z bigComplex) Div(x, y bigComplex) (bigComplex, error) {
	// (a+bi) / (c+di) = (a+bi)(c-di) / (c+di)(c-di)
	//                 = ((ac+bd) + (bc-ad)i) / (cc+dd)
	//                 = (ac+bd)/(cc+dd) + ((bc-ad)/(cc+dd))i
	a, b, c, d := x.re, x.im, y.re, y.im
	ac := new(big.Rat).Mul(a, c)
	ad := new(big.Rat).Mul(a, d)
	bc := new(big.Rat).Mul(b, c)
	bd := new(big.Rat).Mul(b, d)
	cc := new(big.Rat).Mul(c, c)
	dd := new(big.Rat).Mul(d, d)
	ccdd := new(big.Rat).Add(cc, dd)
	if ccdd.Cmp(bigRatZero) == 0 {
		return bigComplex{}, errDivZero
	}
	z.re.Add(ac, bd).Quo(z.re, ccdd)
	z.im.Sub(bc, ad).Quo(z.im, ccdd)
	return z, nil
}
