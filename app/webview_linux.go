//go:build !crossbuild

package main

/*
#cgo pkg-config: webkit2gtk-4.0 gtk+-3.0
#cgo CFLAGS: -w

#include <webkit2/webkit2.h>
#include <gtk/gtk.h>

// Grant all permission requests (camera, microphone, etc.) so getUserMedia
// works inside the Wails webview on Linux.
static gboolean grant_permission(WebKitWebView *web_view,
    WebKitPermissionRequest *request, gpointer user_data) {
    webkit_permission_request_allow(request);
    return TRUE;
}

// Recursively search the widget tree for the WebKitWebView and connect the
// permission-request signal handler.
static void find_webview_cb(GtkWidget *widget, gpointer data) {
    gboolean *found = (gboolean *)data;
    if (*found) return;

    if (WEBKIT_IS_WEB_VIEW(widget)) {
        g_signal_connect(widget, "permission-request",
                         G_CALLBACK(grant_permission), NULL);
        *found = TRUE;
        return;
    }
    if (GTK_IS_CONTAINER(widget)) {
        gtk_container_forall(GTK_CONTAINER(widget), find_webview_cb, data);
    }
}

// Idle callback — runs on the GTK main loop after Wails has created the
// window and webview.  Keeps retrying until the WebKitWebView is found.
static gboolean try_hook_permissions(gpointer data) {
    GList *windows = gtk_window_list_toplevels();
    if (!windows) return G_SOURCE_CONTINUE;

    gboolean found = FALSE;
    for (GList *l = windows; l != NULL; l = l->next) {
        find_webview_cb(GTK_WIDGET(l->data), &found);
        if (found) break;
    }
    g_list_free(windows);
    return found ? G_SOURCE_REMOVE : G_SOURCE_CONTINUE;
}

static void schedulePermissionHook(void) {
    g_idle_add(try_hook_permissions, NULL);
}
*/
import "C"

func init() {
	C.schedulePermissionHook()
}
