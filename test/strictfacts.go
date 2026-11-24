// errorcheckdir

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Function facts cross packages: a strict file uses what the bodies of the
// functions it imports prove. A call of a function that never returns ends the
// flow, and the result of a function that never returns nil needs no check.
// Anything the facts do not prove stays nullable. An argument that may be nil
// for a parameter that the function dereferences is a warning.

package ignored
