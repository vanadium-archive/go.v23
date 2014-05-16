package val

import (
	"math/big"
)

// bigComplex represents a constant complex number.  The semantics are similar
// to big.Rat; methods are typically of the form:
//   func (z *bigComplex) Op(x, y *bigComplex) *bigComplex
// and implement operations z = x Op y with the result as receiver.
type bigComplex struct {
	re, im big.Rat
}

func newComplex(re, im *big.Rat) *bigComplex {
	return &bigComplex{*re, *im}
}

// realComplex returns a bigComplex with real part re, and imaginary part zero.
func realComplex(re *big.Rat) *bigComplex {
	return &bigComplex{re: *re}
}

// imagComplex returns a bigComplex with real part zero, and imaginary part im.
func imagComplex(im *big.Rat) *bigComplex {
	return &bigComplex{im: *im}
}

func (z *bigComplex) SetComplex128(c complex128) *bigComplex {
	z.re.SetFloat64(real(c))
	z.im.SetFloat64(imag(c))
	return z
}

func (z *bigComplex) Equal(x *bigComplex) bool {
	return z.re.Cmp(&x.re) == 0 && z.im.Cmp(&x.im) == 0
}

func (z *bigComplex) Add(x, y *bigComplex) *bigComplex {
	z.re.Add(&x.re, &y.re)
	z.im.Add(&x.im, &y.im)
	return z
}

func (z *bigComplex) Sub(x, y *bigComplex) *bigComplex {
	z.re.Sub(&x.re, &y.re)
	z.im.Sub(&x.im, &y.im)
	return z
}

func (z *bigComplex) Neg(x *bigComplex) *bigComplex {
	z.re.Neg(&x.re)
	z.im.Neg(&x.im)
	return z
}

func (z *bigComplex) Mul(x, y *bigComplex) *bigComplex {
	// (a+bi) * (c+di) = (ac-bd) + (bc+ad)i
	var ac, ad, bc, bd big.Rat
	ac.Mul(&x.re, &y.re)
	ad.Mul(&x.re, &y.im)
	bc.Mul(&x.im, &y.re)
	bd.Mul(&x.im, &y.im)
	z.re.Sub(&ac, &bd)
	z.im.Add(&bc, &ad)
	return z
}

func (z *bigComplex) Div(x, y *bigComplex) (*bigComplex, error) {
	// (a+bi) / (c+di) = (a+bi)(c-di) / (c+di)(c-di)
	//                 = ((ac+bd) + (bc-ad)i) / (cc+dd)
	//                 = (ac+bd)/(cc+dd) + ((bc-ad)/(cc+dd))i
	a, b, c, d := &x.re, &x.im, &y.re, &y.im
	var ac, ad, bc, bd, cc, dd, ccdd big.Rat
	ac.Mul(a, c)
	ad.Mul(a, d)
	bc.Mul(b, c)
	bd.Mul(b, d)
	cc.Mul(c, c)
	dd.Mul(d, d)
	ccdd.Add(&cc, &dd)
	if ccdd.Cmp(bigRatZero) == 0 {
		return nil, errDivZero
	}
	z.re.Add(&ac, &bd).Quo(&z.re, &ccdd)
	z.im.Sub(&bc, &ad).Quo(&z.im, &ccdd)
	return z, nil
}
