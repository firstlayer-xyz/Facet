//go:build !crossbuild

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework WebKit -framework Foundation -lobjc

#import <objc/runtime.h>
#import <WebKit/WebKit.h>

// Grant all media-capture requests (camera) so getUserMedia works in the webview.
static void grantMediaCapture(id self, SEL _cmd, WKWebView *webView,
    WKSecurityOrigin *origin, WKFrameInfo *frame, WKMediaCaptureType type,
    void (^decisionHandler)(WKPermissionDecision)) {
    decisionHandler(WKPermissionDecisionGrant);
}

static void injectMediaCapturePermission(void) {
    Class cls = NSClassFromString(@"WailsContext");
    if (!cls) return;

    SEL sel = @selector(webView:requestMediaCapturePermissionForOrigin:initiatedByFrame:type:decisionHandler:);
    if (class_respondsToSelector(cls, sel)) return; // already implemented

    class_addMethod(cls, sel, (IMP)grantMediaCapture, "v@:@@@q@?");
}
*/
import "C"

func init() {
	C.injectMediaCapturePermission()
}
