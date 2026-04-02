#include "deps/quickjs/quickjs.c"

int QuickjsGoPollInterrupt(JSContext *ctx) {
	return __js_poll_interrupts(ctx);
}
