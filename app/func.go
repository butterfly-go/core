package app

import "slices"

type Func struct {
	fn    func() error
	order int
}

var funcs = make([]Func, 0)

func NewFn(fn func() error) Func {
	return Func{
		fn: fn,
	}
}

func NewFnWithOrder(fn func() error, order int) Func {
	return Func{
		fn:    fn,
		order: order,
	}
}

func appendFn(fn ...Func) {
	funcs = append(funcs, fn...)
}

func do() error {
	slices.SortFunc(funcs, func(a, b Func) int {
		return a.order - b.order
	})

	for _, f := range funcs {
		err := f.fn()
		if err != nil {
			return err
		}
	}
	return nil
}
