//go:build darwin && !crossbuild

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework ScreenCaptureKit -framework AVFoundation -framework CoreMedia -framework ImageIO -framework CoreGraphics

#import <Foundation/Foundation.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <AVFoundation/AVFoundation.h>
#import <ImageIO/ImageIO.h>
#import <CoreGraphics/CoreGraphics.h>
#import <string.h>

// Single active recording. ScreenCaptureKit's SCStream + SCRecordingOutput
// (macOS 15+) write the file for us, so we only hold the stream to stop it.
static SCStream *gStream = nil;
// pid whose windows the active composite capture includes; lets a later
// facet_capture_add_app rebuild the filter as Facet's windows + a launched app's.
static int gCapturePid = 0;
// Clean-composite background colour. SCStreamConfiguration.backgroundColor is an
// assign (non-retaining) property, so the CGColor must outlive the config —
// create it once for the app's lifetime rather than releasing it after assign.
static CGColorRef gBgColor = NULL;

static void fcSetErr(char *errbuf, int errlen, NSString *msg) {
    if (errbuf == NULL || errlen <= 0) return;
    const char *c = [msg UTF8String];
    if (c == NULL) c = "";
    strncpy(errbuf, c, (size_t)errlen - 1);
    errbuf[errlen - 1] = '\0';
}

// fcShareableContent synchronously returns the current shareable content (the
// SCShareableContent API is async; we block on it). *outErr set on a permission
// failure. Returns nil on error.
API_AVAILABLE(macos(12.3))
static SCShareableContent *fcShareableContent(NSError **outErr) {
    __block SCShareableContent *result = nil;
    __block NSError *listErr = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
        result = content;
        listErr = error;
        dispatch_semaphore_signal(sem);
    }];
    dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
    if (outErr) *outErr = listErr;
    return result;
}

// fcCollectWindows gathers on-screen, non-trivial windows either owned by `pid`
// (when pid>0) or by an app whose name contains `appName` (when non-nil). Used to
// build an includingWindows filter that composites only the app windows we want
// (Facet + a launched slicer) onto a clean background — no desktop.
API_AVAILABLE(macos(12.3))
static NSArray<SCWindow *> *fcCollectWindows(SCShareableContent *content, int pid, NSString *appName) {
    NSMutableArray<SCWindow *> *ws = [NSMutableArray array];
    if (content == nil) return ws;
    for (SCWindow *w in content.windows) {
        if (w.owningApplication == nil || !w.onScreen) continue;
        BOOL match = (pid > 0 && (int)w.owningApplication.processID == pid);
        if (!match && appName != nil) {
            NSString *an = w.owningApplication.applicationName;
            if (an != nil && [an localizedCaseInsensitiveContainsString:appName]) match = YES;
        }
        // Skip splash/utility panels so the composite is just the real windows.
        if (match && w.frame.size.width >= 200 && w.frame.size.height >= 150) {
            [ws addObject:w];
        }
    }
    return ws;
}

// fcFindAppWindow returns the on-screen window owned by pid with the largest
// area (the main app window), or nil. getShareableContent is async; we block on
// it. *outErr is set on a TCC/permission failure.
API_AVAILABLE(macos(12.3))
static SCWindow *fcFindAppWindow(int pid, NSError **outErr) {
    __block SCWindow *target = nil;
    __block NSError *listErr = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
        if (error != nil) {
            listErr = error;
        } else {
            CGFloat bestArea = 0;
            for (SCWindow *w in content.windows) {
                if (w.owningApplication == nil) continue;
                if ((int)w.owningApplication.processID != pid) continue;
                if (!w.onScreen) continue;
                CGFloat area = w.frame.size.width * w.frame.size.height;
                if (area > bestArea) { bestArea = area; target = w; }
            }
        }
        dispatch_semaphore_signal(sem);
    }];
    dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
    if (outErr) *outErr = listErr;
    return target;
}

// facet_capture_window_png grabs a still image of the app's own window to a PNG
// at outPath. Returns 0 on success; non-zero with errbuf populated.
int facet_capture_window_png(int pid, const char *outPath, char *errbuf, int errlen) {
    if (@available(macOS 14.0, *)) {
        NSError *listErr = nil;
        SCWindow *target = fcFindAppWindow(pid, &listErr);
        if (listErr != nil) {
            fcSetErr(errbuf, errlen, [NSString stringWithFormat:@"screen capture unavailable — grant Facet 'Screen Recording' in System Settings > Privacy & Security (%@)", listErr.localizedDescription]);
            return 1;
        }
        if (target == nil) { fcSetErr(errbuf, errlen, @"could not find the app window"); return 1; }

        SCContentFilter *filter = [[SCContentFilter alloc] initWithDesktopIndependentWindow:target];
        SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
        cfg.width = (size_t)(filter.contentRect.size.width * filter.pointPixelScale);
        cfg.height = (size_t)(filter.contentRect.size.height * filter.pointPixelScale);
        // Keep the real OS cursor out of screenshots (see the video path).
        cfg.showsCursor = NO;

        __block CGImageRef img = NULL;
        __block NSError *capErr = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [SCScreenshotManager captureImageWithFilter:filter configuration:cfg completionHandler:^(CGImageRef image, NSError *error) {
            if (error != nil) capErr = error;
            else if (image != NULL) img = CGImageRetain(image);
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
        if (capErr != nil || img == NULL) {
            fcSetErr(errbuf, errlen, capErr ? [NSString stringWithFormat:@"capture image: %@", capErr.localizedDescription] : @"capture image: no image");
            return 1;
        }

        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:outPath]];
        CGImageDestinationRef dest = CGImageDestinationCreateWithURL((__bridge CFURLRef)url, (__bridge CFStringRef)@"public.png", 1, NULL);
        if (dest == NULL) { CGImageRelease(img); fcSetErr(errbuf, errlen, @"create PNG destination"); return 1; }
        CGImageDestinationAddImage(dest, img, NULL);
        bool ok = CGImageDestinationFinalize(dest);
        CFRelease(dest);
        CGImageRelease(img);
        if (!ok) { fcSetErr(errbuf, errlen, @"write PNG"); return 1; }
        return 0;
    } else {
        fcSetErr(errbuf, errlen, @"window screenshot requires macOS 14 or later");
        return 1;
    }
}

// fcStartCapture configures an SCStream from `filter`, wires an mp4 recording
// output to outPath, and starts it. width/height (both >0) set the output pixel
// size; otherwise the filter's native size is used. showsCursor controls whether
// the hardware pointer is composited: NO for window/page shots (driven by
// synthetic events, so the real pointer is stale), YES for a whole-screen shot
// where the real pointer IS the action. On success stores the stream in gStream
// and returns 0; otherwise sets errbuf and returns non-zero.
API_AVAILABLE(macos(15.0))
static int fcStartCapture(SCContentFilter *filter, const char *outPath, int width, int height, BOOL showsCursor, BOOL cleanBg, char *errbuf, int errlen) {
    SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
    if (width > 0 && height > 0) {
        // Caller-chosen output size; scale the content to fit (preserving aspect).
        cfg.width = (size_t)width;
        cfg.height = (size_t)height;
        cfg.scalesToFit = YES;
    } else {
        cfg.width = (size_t)(filter.contentRect.size.width * filter.pointPixelScale);
        cfg.height = (size_t)(filter.contentRect.size.height * filter.pointPixelScale);
    }
    cfg.minimumFrameInterval = CMTimeMake(1, 60);
    cfg.showsCursor = showsCursor;
    if (cleanBg) {
        // An includingWindows filter composites only the chosen windows; fill the
        // uncovered area with a neutral colour so the desktop/wallpaper/Dock/menu
        // bar never appear. backgroundColor doesn't retain, so keep the CGColor
        // alive for the app's lifetime (created once) instead of releasing it.
        if (gBgColor == NULL) {
            CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
            CGFloat comps[4] = {0.11, 0.11, 0.12, 1.0};
            gBgColor = CGColorCreate(cs, comps);
            CGColorSpaceRelease(cs);
        }
        cfg.backgroundColor = gBgColor;
    }

    SCStream *stream = [[SCStream alloc] initWithFilter:filter configuration:cfg delegate:nil];

    NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:outPath]];
    SCRecordingOutputConfiguration *rcfg = [[SCRecordingOutputConfiguration alloc] init];
    rcfg.outputURL = url;
    rcfg.outputFileType = AVFileTypeMPEG4;
    SCRecordingOutput *rec = [[SCRecordingOutput alloc] initWithConfiguration:rcfg delegate:nil];

    NSError *addErr = nil;
    if (![stream addRecordingOutput:rec error:&addErr]) {
        fcSetErr(errbuf, errlen, [NSString stringWithFormat:@"add recording output: %@", addErr.localizedDescription]);
        return 1;
    }

    __block NSError *startErr = nil;
    dispatch_semaphore_t startSem = dispatch_semaphore_create(0);
    [stream startCaptureWithCompletionHandler:^(NSError *error) {
        startErr = error;
        dispatch_semaphore_signal(startSem);
    }];
    dispatch_semaphore_wait(startSem, DISPATCH_TIME_FOREVER);
    if (startErr != nil) {
        fcSetErr(errbuf, errlen, [NSString stringWithFormat:@"start capture: %@", startErr.localizedDescription]);
        return 1;
    }

    gStream = stream;
    return 0;
}

// fcMainDisplayIn returns the main display within already-fetched content, or nil.
API_AVAILABLE(macos(12.3))
static SCDisplay *fcMainDisplayIn(SCShareableContent *content) {
    if (content == nil) return nil;
    CGDirectDisplayID mainID = CGMainDisplayID();
    for (SCDisplay *d in content.displays) {
        if (d.displayID == mainID) return d;
    }
    return content.displays.firstObject;
}

// facet_start_window_capture records the on-screen window owned by pid to
// outPath (mp4). If width>0 && height>0, the video is scaled to those pixel
// dimensions; otherwise it uses the window's native (Retina) pixel size.
// Returns 0 on success; non-zero with errbuf populated.
int facet_start_window_capture(int pid, const char *outPath, int width, int height, char *errbuf, int errlen) {
    if (@available(macOS 15.0, *)) {
        if (gStream != nil) { fcSetErr(errbuf, errlen, @"already recording"); return 1; }

        // Find the target window: on-screen, owned by our process, largest area
        // (the main app window). getShareableContent is async; block on it.
        __block SCWindow *target = nil;
        __block NSError *listErr = nil;
        dispatch_semaphore_t listSem = dispatch_semaphore_create(0);
        [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
            if (error != nil) {
                listErr = error;
            } else {
                CGFloat bestArea = 0;
                for (SCWindow *w in content.windows) {
                    if (w.owningApplication == nil) continue;
                    if ((int)w.owningApplication.processID != pid) continue;
                    if (!w.onScreen) continue;
                    CGFloat area = w.frame.size.width * w.frame.size.height;
                    if (area > bestArea) { bestArea = area; target = w; }
                }
            }
            dispatch_semaphore_signal(listSem);
        }];
        dispatch_semaphore_wait(listSem, DISPATCH_TIME_FOREVER);

        if (listErr != nil) {
            fcSetErr(errbuf, errlen, [NSString stringWithFormat:@"screen capture unavailable — grant Facet 'Screen Recording' in System Settings > Privacy & Security (%@)", listErr.localizedDescription]);
            return 1;
        }
        if (target == nil) {
            fcSetErr(errbuf, errlen, @"could not find the app window (is Screen Recording permission granted, and the window visible?)");
            return 1;
        }

        SCContentFilter *filter = [[SCContentFilter alloc] initWithDesktopIndependentWindow:target];
        // Window/page shots are driven by synthetic events, so hide the stale
        // real pointer (the DemoCursor DOM element renders into the window pixels).
        return fcStartCapture(filter, outPath, width, height, NO, NO, errbuf, errlen);
    } else {
        fcSetErr(errbuf, errlen, @"window recording requires macOS 15 or later");
        return 1;
    }
}

// facet_start_composite_capture records only the windows owned by `pid` (Facet)
// composited onto a clean background — no desktop/wallpaper/Dock — with the real
// cursor visible. It's the base for a "hand off to another app" shot: start with
// just Facet, then facet_capture_add_app folds the launched app's window in.
// width/height scale the output if both >0. Returns 0 on success.
int facet_start_composite_capture(int pid, const char *outPath, int width, int height, char *errbuf, int errlen) {
    if (@available(macOS 15.0, *)) {
        if (gStream != nil) { fcSetErr(errbuf, errlen, @"already recording"); return 1; }
        NSError *listErr = nil;
        SCShareableContent *content = fcShareableContent(&listErr);
        if (listErr != nil) {
            fcSetErr(errbuf, errlen, [NSString stringWithFormat:@"screen capture unavailable — grant Facet 'Screen Recording' in System Settings > Privacy & Security (%@)", listErr.localizedDescription]);
            return 1;
        }
        SCDisplay *display = fcMainDisplayIn(content);
        if (display == nil) { fcSetErr(errbuf, errlen, @"no display found"); return 1; }
        NSArray<SCWindow *> *wins = fcCollectWindows(content, pid, nil);
        if (wins.count == 0) {
            fcSetErr(errbuf, errlen, @"could not find the Facet window (is it visible?)");
            return 1;
        }
        SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display includingWindows:wins];
        gCapturePid = pid;
        // showsCursor=NO: the visible pointer is the DemoCursor (a DOM element in
        // Facet's window, captured as part of its pixels); the real system cursor
        // would just be a redundant second arrow. cleanBg=YES.
        return fcStartCapture(filter, outPath, width, height, NO, YES, errbuf, errlen);
    } else {
        fcSetErr(errbuf, errlen, @"screen recording requires macOS 15 or later");
        return 1;
    }
}

// facet_capture_add_app folds a launched app's window(s) into the active
// composite capture, so the recording shows Facet + that app on the clean
// background. Polls up to ~30s for the window (the app may still be launching),
// then live-updates the stream's content filter. Returns 0 once added.
int facet_capture_add_app(const char *appNameC, char *errbuf, int errlen) {
    if (@available(macOS 15.0, *)) {
        if (gStream == nil) { fcSetErr(errbuf, errlen, @"not recording"); return 1; }
        NSString *appName = [NSString stringWithUTF8String:appNameC];
        // ~50 polls (getShareableContent + 0.5s each ≈ under the 60s automation
        // timeout). Pre-warm the target app so its window is found on the first
        // poll; this loop is the safety net.
        for (int i = 0; i < 50; i++) {
            SCShareableContent *content = fcShareableContent(NULL);
            // Pick the app's LARGEST window and require it to be main-window-sized
            // — a launch splash/banner is smaller, so this keeps polling past it
            // and folds in the real window instead.
            SCWindow *mainWin = nil;
            CGFloat bestArea = 0;
            for (SCWindow *w in fcCollectWindows(content, 0, appName)) {
                CGFloat area = w.frame.size.width * w.frame.size.height;
                if (area > bestArea) { bestArea = area; mainWin = w; }
            }
            if (mainWin != nil && mainWin.frame.size.width >= 700 && mainWin.frame.size.height >= 450) {
                SCDisplay *display = fcMainDisplayIn(content);
                NSMutableArray<SCWindow *> *all = [NSMutableArray arrayWithArray:fcCollectWindows(content, gCapturePid, nil)];
                [all addObject:mainWin];
                SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display includingWindows:all];
                __block NSError *upErr = nil;
                dispatch_semaphore_t sem = dispatch_semaphore_create(0);
                [gStream updateContentFilter:filter completionHandler:^(NSError *e) {
                    upErr = e;
                    dispatch_semaphore_signal(sem);
                }];
                dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
                if (upErr != nil) {
                    fcSetErr(errbuf, errlen, [NSString stringWithFormat:@"update filter: %@", upErr.localizedDescription]);
                    return 1;
                }
                return 0;
            }
            usleep(500000); // 0.5s between polls
        }
        fcSetErr(errbuf, errlen, [NSString stringWithFormat:@"window for '%@' did not appear", appName]);
        return 1;
    } else {
        fcSetErr(errbuf, errlen, @"screen recording requires macOS 15 or later");
        return 1;
    }
}

// facet_stop_window_capture finalizes the active recording. Returns 0 on success.
int facet_stop_window_capture(char *errbuf, int errlen) {
    if (@available(macOS 15.0, *)) {
        if (gStream == nil) { fcSetErr(errbuf, errlen, @"not recording"); return 1; }
        SCStream *stream = gStream;
        __block NSError *stopErr = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [stream stopCaptureWithCompletionHandler:^(NSError *error) {
            stopErr = error;
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
        gStream = nil;
        if (stopErr != nil) {
            fcSetErr(errbuf, errlen, [NSString stringWithFormat:@"stop capture: %@", stopErr.localizedDescription]);
            return 1;
        }
        return 0;
    } else {
        fcSetErr(errbuf, errlen, @"window recording requires macOS 15 or later");
        return 1;
    }
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// startWindowCapture begins recording the on-screen window owned by pid to
// outPath (an .mp4). width/height set the output pixel size when both > 0;
// otherwise the window's native size is used. Blocks until ScreenCaptureKit has
// started (or errored).
func startWindowCapture(outPath string, pid, width, height int) error {
	cPath := C.CString(outPath)
	defer C.free(unsafe.Pointer(cPath))
	var errbuf [512]C.char
	rc := C.facet_start_window_capture(C.int(pid), cPath, C.int(width), C.int(height), &errbuf[0], C.int(len(errbuf)))
	if rc != 0 {
		return fmt.Errorf("%s", C.GoString(&errbuf[0]))
	}
	return nil
}

// startCompositeCapture begins recording only pid's windows (Facet) composited
// on a clean background — no desktop — with the real cursor visible. width/height
// set the output size when both > 0. Blocks until started. Stopped via
// stopWindowCapture (shared stream). Fold in a launched app with captureAddApp.
func startCompositeCapture(outPath string, pid, width, height int) error {
	cPath := C.CString(outPath)
	defer C.free(unsafe.Pointer(cPath))
	var errbuf [512]C.char
	rc := C.facet_start_composite_capture(C.int(pid), cPath, C.int(width), C.int(height), &errbuf[0], C.int(len(errbuf)))
	if rc != 0 {
		return fmt.Errorf("%s", C.GoString(&errbuf[0]))
	}
	return nil
}

// captureAddApp folds a launched app's window(s) into the active composite
// capture (polls up to ~30s for the window, then live-updates the filter). Used
// after handing a model off — e.g. captureAddApp("BambuStudio").
func captureAddApp(appName string) error {
	cName := C.CString(appName)
	defer C.free(unsafe.Pointer(cName))
	var errbuf [512]C.char
	rc := C.facet_capture_add_app(cName, &errbuf[0], C.int(len(errbuf)))
	if rc != 0 {
		return fmt.Errorf("%s", C.GoString(&errbuf[0]))
	}
	return nil
}

// stopWindowCapture finalizes the active recording so the file is playable.
func stopWindowCapture() error {
	var errbuf [512]C.char
	rc := C.facet_stop_window_capture(&errbuf[0], C.int(len(errbuf)))
	if rc != 0 {
		return fmt.Errorf("%s", C.GoString(&errbuf[0]))
	}
	return nil
}

// captureWindowImage writes a still PNG of the app's own window to outPath.
func captureWindowImage(outPath string, pid int) error {
	cPath := C.CString(outPath)
	defer C.free(unsafe.Pointer(cPath))
	var errbuf [512]C.char
	rc := C.facet_capture_window_png(C.int(pid), cPath, &errbuf[0], C.int(len(errbuf)))
	if rc != 0 {
		return fmt.Errorf("%s", C.GoString(&errbuf[0]))
	}
	return nil
}
