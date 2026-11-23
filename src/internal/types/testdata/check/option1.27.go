// -lang=go1.27

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p

// The predeclared Option, Some, and None need go1.28.

var o Option /* ERROR "predeclared Option requires go1.28" */ [int]
var s = Some /* ERROR "predeclared Some requires go1.28" */ (1)
var n = None /* ERROR "predeclared None requires go1.28" */
