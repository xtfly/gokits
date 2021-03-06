// Copyright 2016 lanlingzi. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

#include "go_asm.h"
#include "go_tls.h"
#include "textflag.h"

TEXT ·Getg(SB), NOSPLIT, $0-8
    get_tls(CX)
    MOVQ    g(CX), AX
    MOVQ    AX, ret+0(FP)
    RET
