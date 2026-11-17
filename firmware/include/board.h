// Selects the board header for the environment being built.
#pragma once

#if defined(ROGER_BOARD_M5STICKS3)
#include "boards/m5sticks3.h"
#elif defined(ROGER_BOARD_XIAO_DIY)
#include "boards/xiao_diy.h"
#else
#error "No board selected. Build with -e m5sticks3 or -e xiao_diy (see platformio.ini)."
#endif
