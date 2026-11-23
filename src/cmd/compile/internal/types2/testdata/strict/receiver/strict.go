//go:strict

package receiver

func (t *T) M() {}

func use() {
	P /* ERROR "strict: cannot use P" */ .M()

	var t T
	t.M()

	p := &T{}
	p.M()
}
