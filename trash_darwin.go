//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>

// recycle trashes the paths "in the same manner as the Finder" (so Put Back
// can work) without needing Automation permission. The completion handler is
// delivered via the main run loop, so this must not be called on the main
// thread; bound service methods run on their own goroutines.
static void recycle(const char** paths, int n) {
	@autoreleasepool {
		NSMutableArray<NSURL*> *urls = [NSMutableArray arrayWithCapacity:n];
		for (int i = 0; i < n; i++) {
			[urls addObject:[NSURL fileURLWithPath:[NSString stringWithUTF8String:paths[i]]]];
		}
		dispatch_semaphore_t done = dispatch_semaphore_create(0);
		[[NSWorkspace sharedWorkspace] recycleURLs:urls completionHandler:^(NSDictionary *moved, NSError *err) {
			dispatch_semaphore_signal(done);
		}];
		dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 60LL * NSEC_PER_SEC));
	}
}

static char* trashItem(const char* path) {
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSError *err = nil;
		if ([[NSFileManager defaultManager] trashItemAtURL:url resultingItemURL:nil error:&err]) {
			return NULL;
		}
		return strdup([[err localizedDescription] UTF8String]);
	}
}
*/
import "C"

import (
	"errors"
	"os"
	"unsafe"
)

func trashPaths(paths []string) map[string]error {
	if len(paths) == 0 {
		return nil
	}
	cpaths := make([]*C.char, len(paths))
	for i, p := range paths {
		cpaths[i] = C.CString(p)
	}
	C.recycle((**C.char)(unsafe.Pointer(&cpaths[0])), C.int(len(cpaths)))
	for _, cp := range cpaths {
		C.free(unsafe.Pointer(cp))
	}

	// Anything NSWorkspace didn't move goes through NSFileManager, which
	// reports a reason on failure (but doesn't record Put Back info).
	failed := map[string]error{}
	for _, p := range paths {
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		cp := C.CString(p)
		msg := C.trashItem(cp)
		C.free(unsafe.Pointer(cp))
		if msg != nil {
			failed[p] = errors.New(C.GoString(msg))
			C.free(unsafe.Pointer(msg))
		}
	}
	return failed
}
