//go:strict

package receiver

func (t *T) M() {}

func use() {
	P /* ERROR "strict: warning: P (variable of type *T) may be nil" */ /* ERROR "strict: cannot use P" */ .M()

	var t T
	t.M()

	p := &T{}
	p.M()
}
