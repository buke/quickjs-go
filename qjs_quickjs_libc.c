#define QJS_BUILD_LIBC
#if defined(_WIN32)
#include <winsock2.h>
#endif
#include "deps/quickjs/quickjs-libc.c"