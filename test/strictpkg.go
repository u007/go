// errorcheckdir

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Strictness crosses packages: a type written in a //go:strict file of one
// package is non-nil in a //go:strict file of another.

package ignored
