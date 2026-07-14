//go:build darwin && !crossbuild

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework ScreenCaptureKit -framework AVFoundation -framework CoreMedia -framework ImageIO -framework CoreGraphics

#import <Foundation/Foundation.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <AVFoundation/AVFoundation.h>
#import <ImageIO/ImageIO.h>
#import <string.h>

// Single active recording. ScreenCaptureKit's SCStream + SCRecordingOutput
// (macOS 15+) write the file for us, so we only hold the stream to stop it.
static SCStream *gStream = nil;

static void fcSetErr(char *errbuf, int errlen, NSString *msg) {
    if (errbuf == NULL || errlen <= 0) return;
    const char *c = [msg UTF8String];
    if (c == NULL) c = "";
    strncpy(errbuf, c, (size_t)errlen - 1);
    errbuf[errlen - 1] = '\0';
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

        SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
        if (width > 0 && height > 0) {
            // Caller-chosen output size; scale the window content to fit it
            // (preserving aspect — letterboxed if it differs from the window).
            cfg.width = (size_t)width;
            cfg.height = (size_t)height;
            cfg.scalesToFit = YES;
        } else {
            // Default: match the window's pixel dimensions so the video is 1:1.
            cfg.width = (size_t)(filter.contentRect.size.width * filter.pointPixelScale);
            cfg.height = (size_t)(filter.contentRect.size.height * filter.pointPixelScale);
        }
        cfg.minimumFrameInterval = CMTimeMake(1, 60);
        cfg.showsCursor = YES;

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
    } else {
        fcSetErr(errbuf, errlen, @"window recording requires macOS 15 or later");
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
