// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package b

// Option, Some, and None declared at package level shadow the predeclared
// names in this package and in every package that imports it.

type Option struct{ v int }

const None = -1

func Some(v int) Option { return Option{v} }

func (o Option) Value() int { return o.v }
